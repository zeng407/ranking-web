package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// OAuth errors. Unlike the login errors these are distinguishable, because the user
// has to be told what to do about them: "this address already has an account" is
// actionable, and there is nothing to enumerate — the caller already proved control
// of the address to Google.
var (
	// ErrOAuthStateInvalid covers an unknown, expired or already-used state. Every
	// one of them means the callback cannot be trusted, so they answer the same way.
	ErrOAuthStateInvalid = errors.New("auth: oauth state is not valid")
	// ErrOAuthEmailTaken means the verified Google address already belongs to a local
	// account that has no Google link. Laravel refuses this too
	// (UserSocialiteEmailExists) and the refusal is deliberate: linking on the
	// strength of a matching address would hand the local account to whoever controls
	// the Google one.
	ErrOAuthEmailTaken = errors.New("auth: the address already has an account")
	// ErrOAuthEmailUnverified means Google did not vouch for the address. Without
	// that, an address is just a string the provider passed along, and matching an
	// existing account on it would be an account takeover.
	ErrOAuthEmailUnverified = errors.New("auth: the provider did not verify the address")
	// ErrOAuthAlreadyLinked is the connect flow's failure: either this Google account
	// is on another user, or this user already has one.
	ErrOAuthAlreadyLinked = errors.New("auth: the account is already linked")
	// ErrOAuthProviderFailed wraps anything the provider did wrong. The detail goes
	// to the log; the client gets a generic failure.
	ErrOAuthProviderFailed = errors.New("auth: the provider request failed")
)

// OAuthStateTTL bounds how long a started flow may sit unfinished.
//
// Ten minutes is long enough to read a consent screen and short enough that a state
// captured from a browser's history is dead by the time it is useful.
const OAuthStateTTL = 10 * time.Minute

// OAuthState is what a flow remembers between the redirect out and the callback back.
//
// It lives server-side rather than in a signed cookie for one specific reason: the
// callback arrives as a cross-site navigation from the provider, and a SameSite=Strict
// cookie is not sent on those. A Lax cookie would be, but then the flow's memory would
// be attached to a cookie an attacker can cause to be sent. Server-side state has
// neither problem.
type OAuthState struct {
	// Verifier is the PKCE code verifier. This is a confidential client, so the
	// client secret already binds the exchange to this server; PKCE additionally binds
	// it to this particular flow, which is what stops an authorization code captured
	// from a redirect from being spent by anyone else.
	Verifier string
	// ReturnTo is where the browser goes after the callback finishes. Validated
	// against an allowlist when the flow starts, never when it finishes: a stored
	// value cannot be tampered with, a callback parameter can.
	ReturnTo string
	// ConnectUserID is set for the "link Google to the account I am already logged
	// into" flow, and zero for a login.
	//
	// It is here rather than read from the session at callback time because there is
	// no session to read: see the type comment on why cookies do not arrive.
	ConnectUserID int64
	CreatedAt     time.Time
}

// OAuthStateStore holds in-flight flows.
//
// Consume must be atomic and one-shot. If two callbacks with the same state can both
// read it, an authorization code replayed against this server would run the flow
// twice.
type OAuthStateStore interface {
	Put(ctx context.Context, key string, state OAuthState, ttl time.Duration) error
	// Consume returns the state and removes it in the same operation. A missing key
	// must return ErrOAuthStateInvalid.
	Consume(ctx context.Context, key string) (OAuthState, error)
}

// OAuthIdentity is what a provider tells us about a person, reduced to what this
// application stores.
//
// Deliberately not the provider's access token. Laravel stores google_token and
// google_refresh_token, and nothing in the codebase ever reads either — the refresh
// token is NULL on all 11,304 rows because it was never requested with
// access_type=offline, and the longest access token in the table is 253 characters in
// a VARCHAR(255). A live token in a table nobody reads is a liability with no
// corresponding use, so the Go path does not keep one.
type OAuthIdentity struct {
	// Subject is the provider's immutable id for the account. What the link is keyed
	// on; the address is not, because people change addresses.
	Subject string
	Email   string
	// EmailVerified is the provider's claim that the person controls the address.
	// Nothing may be matched on an unverified address.
	EmailVerified bool
	Name          string
	AvatarURL     string
}

// OAuthProvider is one identity provider.
type OAuthProvider interface {
	// Name is the provider's key, used in logs and in the state's storage key.
	Name() string
	// AuthorizationURL is where the browser is sent.
	AuthorizationURL(state, codeChallenge string) string
	// Exchange turns an authorization code into an identity. Implementations must
	// verify that what comes back is about the person who consented — see google.go
	// for how the id_token is trusted.
	Exchange(ctx context.Context, code, verifier string) (OAuthIdentity, error)
}

// OAuthService runs the flows a provider is used for.
type OAuthService struct {
	provider OAuthProvider
	states   OAuthStateStore
	social   SocialStore
	sessions *Service
	logger   *slog.Logger
	now      func() time.Time
	// returnAllowlist is the set of URL prefixes a completed flow may send the
	// browser to. An open redirect here would be worth having: the callback is a URL
	// users are trained to follow, and it runs immediately after a login.
	returnAllowlist []string
	defaultReturnTo string
}

// OAuthServiceOptions wires OAuthService.
type OAuthServiceOptions struct {
	Provider        OAuthProvider
	States          OAuthStateStore
	Social          SocialStore
	Sessions        *Service
	Logger          *slog.Logger
	Now             func() time.Time
	ReturnAllowlist []string
	DefaultReturnTo string
}

func NewOAuthService(options OAuthServiceOptions) (*OAuthService, error) {
	if options.Provider == nil {
		return nil, errors.New("auth: oauth provider is required")
	}
	if options.States == nil {
		return nil, errors.New("auth: oauth state store is required")
	}
	if options.Social == nil {
		return nil, errors.New("auth: social store is required")
	}
	if options.Sessions == nil {
		return nil, errors.New("auth: session service is required")
	}
	if options.DefaultReturnTo == "" {
		return nil, errors.New("auth: a default return target is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &OAuthService{
		provider:        options.Provider,
		states:          options.States,
		social:          options.Social,
		sessions:        options.Sessions,
		logger:          logger,
		now:             now,
		returnAllowlist: options.ReturnAllowlist,
		defaultReturnTo: options.DefaultReturnTo,
	}, nil
}

// StartedFlow is what the caller redirects the browser to.
type StartedFlow struct {
	AuthorizationURL string
	State            string
}

// Start begins a login flow. connectUserID is zero for a login, or the id of the
// already-authenticated user for a link.
func (service *OAuthService) Start(
	ctx context.Context, returnTo string, connectUserID int64,
) (StartedFlow, error) {
	stateKey, err := randomURLSafe(32)
	if err != nil {
		return StartedFlow{}, err
	}
	verifier, err := randomURLSafe(32)
	if err != nil {
		return StartedFlow{}, err
	}

	state := OAuthState{
		Verifier:      verifier,
		ReturnTo:      service.resolveReturnTo(returnTo),
		ConnectUserID: connectUserID,
		CreatedAt:     service.now().UTC(),
	}
	if err := service.states.Put(ctx, service.stateKey(stateKey), state, OAuthStateTTL); err != nil {
		return StartedFlow{}, err
	}

	return StartedFlow{
		AuthorizationURL: service.provider.AuthorizationURL(stateKey, pkceChallenge(verifier)),
		State:            stateKey,
	}, nil
}

// CompletedFlow is the result of a callback.
type CompletedFlow struct {
	// Grant is set for a login and empty for a link: linking does not change who is
	// logged in, so it must not mint a session.
	Grant Grant
	// Linked is true when this callback attached a provider account to an existing
	// user rather than logging anyone in.
	Linked bool
	// Created is true when a new account was made, which the caller may want to
	// treat differently — a first-time user has no nickname yet.
	Created  bool
	ReturnTo string
	UserID   int64
}

// Complete finishes a flow from the callback's state and code.
func (service *OAuthService) Complete(
	ctx context.Context, stateKey, code string, client ClientInfo,
) (CompletedFlow, error) {
	if stateKey == "" || code == "" {
		return CompletedFlow{}, ErrOAuthStateInvalid
	}

	state, err := service.states.Consume(ctx, service.stateKey(stateKey))
	if err != nil {
		return CompletedFlow{}, err
	}

	identity, err := service.provider.Exchange(ctx, code, state.Verifier)
	if err != nil {
		service.logger.Error("oauth_exchange_failed", "provider", service.provider.Name(), "error", err)
		return CompletedFlow{}, fmt.Errorf("%w: %v", ErrOAuthProviderFailed, err)
	}
	if identity.Subject == "" {
		return CompletedFlow{}, fmt.Errorf("%w: the provider returned no subject", ErrOAuthProviderFailed)
	}

	if state.ConnectUserID > 0 {
		return service.link(ctx, state, identity)
	}
	return service.login(ctx, state, identity, client)
}

func (service *OAuthService) login(
	ctx context.Context, state OAuthState, identity OAuthIdentity, client ClientInfo,
) (CompletedFlow, error) {
	// The existing link is checked before the address is even looked at: a person who
	// has signed in before must keep working even if they have since changed their
	// Google address, and even if that address now collides with something else.
	credentials, err := service.social.FindByProviderSubject(ctx, service.provider.Name(), identity.Subject)
	switch {
	case err == nil:
		grant, err := service.sessions.grant(ctx, credentials, client)
		if err != nil {
			return CompletedFlow{}, err
		}
		return CompletedFlow{Grant: grant, ReturnTo: state.ReturnTo, UserID: credentials.UserID}, nil
	case !errors.Is(err, ErrUserNotFound):
		return CompletedFlow{}, err
	}

	// From here on the address is load-bearing, so it has to be one the provider
	// vouches for.
	if !identity.EmailVerified || strings.TrimSpace(identity.Email) == "" {
		return CompletedFlow{}, ErrOAuthEmailUnverified
	}

	// An address that already has a local account is refused rather than linked,
	// matching Laravel. See ErrOAuthEmailTaken.
	taken, err := service.social.EmailExists(ctx, identity.Email)
	if err != nil {
		return CompletedFlow{}, err
	}
	if taken {
		service.logger.Info("oauth_email_already_registered",
			"provider", service.provider.Name(), "subject", identity.Subject)
		return CompletedFlow{}, ErrOAuthEmailTaken
	}

	credentials, err = service.social.CreateLinkedUser(ctx, NewLinkedUser{
		Provider:      service.provider.Name(),
		Subject:       identity.Subject,
		Email:         identity.Email,
		Name:          DisplayNameFromIdentity(identity),
		AvatarURL:     identity.AvatarURL,
		EmailVerified: identity.EmailVerified,
		CreatedAt:     service.now().UTC(),
	})
	if errors.Is(err, ErrOAuthAlreadyLinked) {
		// Two first-time sign-ins for the same Google account at once: the other one
		// won the unique index. The row it created is the row this flow wanted, so the
		// lookup is retried instead of failing — otherwise one of two people clicking
		// the same consent button would be told their account is "already linked" on
		// their very first login.
		credentials, err = service.social.FindByProviderSubject(
			ctx, service.provider.Name(), identity.Subject)
		if err != nil {
			return CompletedFlow{}, err
		}
		grant, err := service.sessions.grant(ctx, credentials, client)
		if err != nil {
			return CompletedFlow{}, err
		}
		return CompletedFlow{Grant: grant, ReturnTo: state.ReturnTo, UserID: credentials.UserID}, nil
	}
	if err != nil {
		return CompletedFlow{}, err
	}

	grant, err := service.sessions.grant(ctx, credentials, client)
	if err != nil {
		return CompletedFlow{}, err
	}
	service.logger.Info("oauth_account_created",
		"provider", service.provider.Name(), "user_id", credentials.UserID)
	return CompletedFlow{
		Grant: grant, Created: true, ReturnTo: state.ReturnTo, UserID: credentials.UserID,
	}, nil
}

func (service *OAuthService) link(
	ctx context.Context, state OAuthState, identity OAuthIdentity,
) (CompletedFlow, error) {
	// No session is issued and none is required: the caller was already
	// authenticated when the flow started, which is how ConnectUserID got set.
	if err := service.social.Link(ctx, LinkRequest{
		UserID:   state.ConnectUserID,
		Provider: service.provider.Name(),
		Subject:  identity.Subject,
		Email:    identity.Email,
	}); err != nil {
		return CompletedFlow{}, err
	}
	service.logger.Info("oauth_account_linked",
		"provider", service.provider.Name(), "user_id", state.ConnectUserID)
	return CompletedFlow{
		Linked: true, ReturnTo: state.ReturnTo, UserID: state.ConnectUserID,
	}, nil
}

// resolveReturnTo keeps a completed login from being an open redirect.
//
// Anything not on the allowlist becomes the default rather than an error: a bad
// return target is not a reason to refuse a login, it is a reason to ignore the
// target.
func (service *OAuthService) resolveReturnTo(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return service.defaultReturnTo
	}
	for _, allowed := range service.returnAllowlist {
		if allowed != "" && strings.HasPrefix(candidate, allowed) {
			return candidate
		}
	}
	service.logger.Warn("oauth_return_target_rejected", "candidate", candidate)
	return service.defaultReturnTo
}

// stateKey namespaces the store by provider, so a state started for one provider
// cannot be presented to another's callback.
func (service *OAuthService) stateKey(key string) string {
	return service.provider.Name() + ":" + key
}

// DisplayNameFromIdentity picks the name a new account starts with.
//
// Truncated to 20 characters, which is setting.user_name_max_size on the PHP side and
// what the existing 11,304 rows were created under. Counted in runes, not bytes: the
// column is utf8mb4 and most of these names are Chinese, where a byte limit would cut
// a character in half.
func DisplayNameFromIdentity(identity OAuthIdentity) string {
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		// The local part of the address, as a last resort. Never empty in practice,
		// but users.name is NOT NULL, so there has to be something.
		if at := strings.IndexByte(identity.Email, '@'); at > 0 {
			name = identity.Email[:at]
		}
	}
	if name == "" {
		name = "user"
	}
	runes := []rune(name)
	if len(runes) > MaxDisplayNameRunes {
		return string(runes[:MaxDisplayNameRunes])
	}
	return name
}

// MaxDisplayNameRunes mirrors config('setting.user_name_max_size').
const MaxDisplayNameRunes = 20

// pkceChallenge is the S256 transform. Plain is not offered: it would make the
// challenge equal to the verifier and remove the point of sending one.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLSafe(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
