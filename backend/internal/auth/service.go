package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is the single answer to every failed login: unknown
// address, wrong password, or an account that has no password at all. One error for
// all three is deliberate — distinguishing them turns the form into an account
// enumeration oracle.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// timingHash is compared against when no account matches, so a request for an
// unregistered address costs the same bcrypt work as a real one. Without it the
// response time alone reveals which addresses exist.
//
// Generated once at startup rather than being a constant, so it is never a value an
// attacker can recognise.
var timingHash []byte

func init() {
	// Cost 10, matching config/hashing.php, so the decoy costs what a real compare
	// costs. An error here cannot be handled at init; the nil hash still fails every
	// compare, which is the safe direction.
	timingHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalisation-placeholder"), 10)
}

// Session lifetimes and limits.
const (
	// MaxPasswordBytes bounds what is fed to bcrypt. bcrypt silently truncates at 72
	// bytes, so a longer input is not more secure and a very long one is only a way
	// to make the server do pointless work.
	MaxPasswordBytes = 72
	// MaxEmailBytes is the users.email column width.
	MaxEmailBytes = 255
)

// Service performs login, refresh and logout.
type Service struct {
	users UserStore
	// registrations creates password accounts. Separate from users because reading an
	// account and creating one are different privileges, and only one endpoint needs
	// the second. Nil disables Register rather than the whole service.
	registrations UserWriter
	// accounts reads and writes the settings a signed-in user owns. Nil disables the
	// account endpoints, in the same way a nil registrations disables Register.
	accounts AccountStore
	avatars  AvatarStore
	sessions RefreshStore
	issuer   *Issuer
	logger   *slog.Logger
	now      func() time.Time
	// timezone is the application timezone, Asia/Taipei, used by the rules that are
	// written against calendar days rather than durations. Nil means UTC.
	timezone *time.Location
	// refreshTTL is how long a session survives without use.
	refreshTTL time.Duration
}

// ServiceOptions wires Service.
type ServiceOptions struct {
	Users      UserStore
	Registrar  UserWriter
	Accounts   AccountStore
	Avatars    AvatarStore
	Sessions   RefreshStore
	Issuer     *Issuer
	Logger     *slog.Logger
	Now        func() time.Time
	RefreshTTL time.Duration
	Timezone   *time.Location
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Users == nil {
		return nil, errors.New("auth: user store is required")
	}
	if options.Sessions == nil {
		return nil, errors.New("auth: refresh store is required")
	}
	if options.Issuer == nil {
		return nil, errors.New("auth: issuer is required")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ttl := options.RefreshTTL
	if ttl <= 0 {
		ttl = RefreshTokenTTL
	}
	return &Service{
		users:         options.Users,
		registrations: options.Registrar,
		accounts:      options.Accounts,
		avatars:       options.Avatars,
		sessions:      options.Sessions,
		issuer:        options.Issuer,
		logger:        logger,
		now:           now,
		refreshTTL:    ttl,
		timezone:      options.Timezone,
	}, nil
}

// Grant is a completed login or refresh.
type Grant struct {
	Access  AccessToken
	Refresh IssuedRefresh
	UserID  int64
	Roles   []string
}

// ClientInfo is recorded for audit only.
type ClientInfo struct {
	IP        string
	UserAgent string
}

// Login exchanges an e-mail address and password for a session.
func (service *Service) Login(ctx context.Context, email, password string, client ClientInfo) (Grant, error) {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > MaxEmailBytes || password == "" || len(password) > MaxPasswordBytes {
		// Still pay the bcrypt cost so a malformed request is not measurably faster
		// than a well-formed one against a real account.
		_ = bcrypt.CompareHashAndPassword(timingHash, []byte(password))
		return Grant{}, ErrInvalidCredentials
	}

	credentials, err := service.users.FindByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(timingHash, []byte(password))
		return Grant{}, ErrInvalidCredentials
	}
	if err != nil {
		return Grant{}, err
	}

	// THE 11,040 ACCOUNTS WITH NO PASSWORD.
	//
	// Most accounts in production signed up through Google or Twitch and their
	// password column is an empty string. bcrypt already rejects an empty hash — it
	// is too short to parse — but relying on that is relying on an error path. This
	// is the explicit guard, so that a later "skip the compare when the hash is
	// empty" optimisation cannot turn into eleven thousand accounts that anyone can
	// log into with any password.
	if strings.TrimSpace(credentials.PasswordHash) == "" {
		_ = bcrypt.CompareHashAndPassword(timingHash, []byte(password))
		return Grant{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(password)); err != nil {
		return Grant{}, ErrInvalidCredentials
	}

	return service.grant(ctx, credentials, client)
}

// Refresh rotates a session.
//
// The presented token is consumed and a new one issued in the same family. A token
// that has already been consumed is treated as theft: there is no way to tell the
// attacker's copy from the victim's, so the family is revoked and both are forced to
// log in again.
func (service *Service) Refresh(ctx context.Context, refreshToken, csrfToken string, client ClientInfo) (Grant, error) {
	if refreshToken == "" {
		return Grant{}, ErrRefreshTokenInvalid
	}

	session, err := service.sessions.FindByTokenHash(ctx, HashToken(refreshToken))
	if err != nil {
		return Grant{}, err
	}

	// CSRF is checked before anything else acts on the session, because the cookie
	// alone arrives on a cross-site request too.
	if err := CheckCSRF(csrfToken, session.CSRFHash); err != nil {
		return Grant{}, err
	}

	now := service.now().UTC()

	if session.Used() {
		// Replay. Revoke everything in the chain, including the token the attacker
		// may already hold.
		revoked, revokeErr := service.sessions.RevokeFamily(ctx, session.FamilyID, now)
		service.logger.Warn("refresh_token_replayed",
			"user_id", session.UserID, "family_id", session.FamilyID,
			"revoked_sessions", revoked, "revoke_error", revokeErr)
		return Grant{}, ErrRefreshTokenReused
	}
	if session.Revoked() || !now.Before(session.ExpiresAt) {
		return Grant{}, ErrRefreshTokenInvalid
	}

	// Consume before issuing. If the process dies between the two the user has to log
	// in again, which is recoverable; issuing first and failing to consume would leave
	// a token that can be replayed, which is not.
	if err := service.sessions.MarkUsed(ctx, session.ID, now); err != nil {
		return Grant{}, err
	}

	credentials, err := service.users.FindByID(ctx, session.UserID)
	if errors.Is(err, ErrUserNotFound) {
		// The account was deleted while the session lived.
		return Grant{}, ErrRefreshTokenInvalid
	}
	if err != nil {
		return Grant{}, err
	}

	// Roles are re-read rather than carried over: a ban applied during the session
	// has to take effect on the next refresh rather than at the next login.
	return service.grantInFamily(ctx, credentials, session.FamilyID, client)
}

// Logout revokes the whole family the token belongs to, so every rotation of that
// login ends, not just the token presented.
func (service *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	session, err := service.sessions.FindByTokenHash(ctx, HashToken(refreshToken))
	if errors.Is(err, ErrRefreshTokenInvalid) {
		// Logging out an unknown token is not an error: the caller wanted the session
		// gone and it is gone.
		return nil
	}
	if err != nil {
		return err
	}
	_, err = service.sessions.RevokeFamily(ctx, session.FamilyID, service.now().UTC())
	return err
}

// RevokeAll ends every session a user holds. For a ban, or a password change.
func (service *Service) RevokeAll(ctx context.Context, userID int64) (int64, error) {
	return service.sessions.RevokeUser(ctx, userID, service.now().UTC())
}

func (service *Service) grant(ctx context.Context, credentials Credentials, client ClientInfo) (Grant, error) {
	familyID, err := NewFamilyID()
	if err != nil {
		return Grant{}, err
	}
	return service.grantInFamily(ctx, credentials, familyID, client)
}

func (service *Service) grantInFamily(
	ctx context.Context, credentials Credentials, familyID string, client ClientInfo,
) (Grant, error) {
	access, err := service.issuer.Issue(credentials.UserID, credentials.Roles)
	if err != nil {
		return Grant{}, err
	}

	refreshToken, refreshHash, err := NewRefreshToken()
	if err != nil {
		return Grant{}, err
	}
	csrfToken, csrfHash, err := NewCSRFToken()
	if err != nil {
		return Grant{}, err
	}

	now := service.now().UTC()
	expiresAt := now.Add(service.refreshTTL)
	if _, err := service.sessions.Create(ctx, NewSession{
		UserID:    credentials.UserID,
		FamilyID:  familyID,
		TokenHash: refreshHash,
		CSRFHash:  csrfHash,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		CreatedIP: client.IP,
		UserAgent: client.UserAgent,
	}); err != nil {
		return Grant{}, err
	}

	return Grant{
		Access: access,
		Refresh: IssuedRefresh{
			Token:     refreshToken,
			CSRFToken: csrfToken,
			FamilyID:  familyID,
			ExpiresAt: expiresAt,
		},
		UserID: credentials.UserID,
		Roles:  credentials.Roles,
	}, nil
}

// SubjectToUserID parses the `sub` claim back into a user id.
func SubjectToUserID(subject string) (int64, error) {
	userID, err := strconv.ParseInt(strings.TrimSpace(subject), 10, 64)
	if err != nil || userID <= 0 {
		return 0, fmt.Errorf("auth: subject %q is not a user id", subject)
	}
	return userID, nil
}

// VerifyCSRF checks an echoed CSRF value against the session a refresh token names.
//
// Exposed for the logout path, which has to reject a forged request without going
// through a rotation. Failures are the same errors the refresh path returns, so the
// transport layer answers identically either way.
func (service *Service) VerifyCSRF(ctx context.Context, refreshToken, csrfToken string) error {
	if refreshToken == "" {
		return ErrRefreshTokenInvalid
	}
	session, err := service.sessions.FindByTokenHash(ctx, HashToken(refreshToken))
	if err != nil {
		return err
	}
	return CheckCSRF(csrfToken, session.CSRFHash)
}
