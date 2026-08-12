package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Google's endpoints. Hard-coded rather than read from the discovery document: they
// have not changed in the lifetime of the protocol, and a startup dependency on a
// network fetch would make the API fail to boot when Google is slow.
const (
	googleAuthorizeEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint     = "https://oauth2.googleapis.com/token"
	googleIssuer            = "https://accounts.google.com"
)

// googleScopes is the minimum that yields an id_token with an address.
//
// openid gives the subject, email gives the address and its verified flag, profile
// gives the display name and picture. No offline access: a refresh token would let
// this server act as the user against Google later, which nothing here does. Laravel
// did not request it either — which is why google_refresh_token is NULL on all 11,304
// rows.
var googleScopes = []string{"openid", "email", "profile"}

// GoogleProvider implements OAuthProvider.
type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	client       *http.Client
	// tokenEndpoint is overridable so tests can point the exchange at a local server.
	tokenEndpoint string
	now           func() time.Time
}

// GoogleConfig wires GoogleProvider.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL must be registered in the Google Cloud console exactly, including
	// the scheme and any port. Google compares it as a string.
	RedirectURL string
	// HTTPClient is optional. The default has a timeout, which http.DefaultClient
	// does not: a token exchange that hangs would hold the request until the client
	// gives up.
	HTTPClient    *http.Client
	TokenEndpoint string
	Now           func() time.Time
}

func NewGoogleProvider(configuration GoogleConfig) (*GoogleProvider, error) {
	if strings.TrimSpace(configuration.ClientID) == "" {
		return nil, errors.New("auth: google client id is required")
	}
	if strings.TrimSpace(configuration.ClientSecret) == "" {
		return nil, errors.New("auth: google client secret is required")
	}
	if strings.TrimSpace(configuration.RedirectURL) == "" {
		return nil, errors.New("auth: google redirect url is required")
	}
	client := configuration.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	endpoint := configuration.TokenEndpoint
	if endpoint == "" {
		endpoint = googleTokenEndpoint
	}
	now := configuration.Now
	if now == nil {
		now = time.Now
	}
	return &GoogleProvider{
		clientID:      configuration.ClientID,
		clientSecret:  configuration.ClientSecret,
		redirectURL:   configuration.RedirectURL,
		client:        client,
		tokenEndpoint: endpoint,
		now:           now,
	}, nil
}

func (provider *GoogleProvider) Name() string { return ProviderGoogle }

func (provider *GoogleProvider) AuthorizationURL(state, codeChallenge string) string {
	query := url.Values{
		"client_id":     {provider.clientID},
		"redirect_uri":  {provider.redirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(googleScopes, " ")},
		"state":         {state},
		// PKCE. See OAuthState.Verifier for why a confidential client sends it too.
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		// select_account rather than none or consent: a shared browser must not sign
		// in as whoever was there last, and forcing the consent screen on every login
		// would be noise for a returning user.
		"prompt": {"select_account"},
	}
	return googleAuthorizeEndpoint + "?" + query.Encode()
}

// googleTokenResponse is the token endpoint's answer. Only the id_token is used; see
// Exchange.
type googleTokenResponse struct {
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// googleIDClaims is the subset of the id_token this application reads.
type googleIDClaims struct {
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Email    string `json:"email"`
	// EmailVerified arrives as a bool from Google. Some providers send the string
	// "true"; json.Unmarshal into bool would fail on that, so it is decoded loosely.
	EmailVerified flexibleBool `json:"email_verified"`
	Name          string       `json:"name"`
	Picture       string       `json:"picture"`
}

func (provider *GoogleProvider) Exchange(
	ctx context.Context, code, verifier string,
) (OAuthIdentity, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {provider.clientID},
		"client_secret": {provider.clientSecret},
		"redirect_uri":  {provider.redirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		provider.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthIdentity{}, fmt.Errorf("build token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := provider.client.Do(request)
	if err != nil {
		return OAuthIdentity{}, fmt.Errorf("call the token endpoint: %w", err)
	}
	defer response.Body.Close()

	// Bounded: a compromised or misbehaving endpoint must not be able to make this
	// process read an unbounded body into memory.
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return OAuthIdentity{}, fmt.Errorf("read the token response: %w", err)
	}

	var token googleTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return OAuthIdentity{}, fmt.Errorf("decode the token response (status %d): %w",
			response.StatusCode, err)
	}
	if response.StatusCode != http.StatusOK || token.Error != "" {
		// The description is Google's, and it says things like "invalid_grant" for a
		// replayed code. Useful in a log, never shown to the caller.
		return OAuthIdentity{}, fmt.Errorf("token endpoint returned %d: %s %s",
			response.StatusCode, token.Error, token.ErrorDescription)
	}
	if token.IDToken == "" {
		return OAuthIdentity{}, errors.New("the token response carried no id_token")
	}

	claims, err := provider.claimsFromIDToken(token.IDToken)
	if err != nil {
		return OAuthIdentity{}, err
	}

	return OAuthIdentity{
		Subject:       claims.Subject,
		Email:         strings.TrimSpace(claims.Email),
		EmailVerified: bool(claims.EmailVerified),
		Name:          claims.Name,
		AvatarURL:     claims.Picture,
	}, nil
}

// claimsFromIDToken reads the identity out of the id_token.
//
// # WHY THE SIGNATURE IS NOT VERIFIED HERE
//
// This token did not come from the browser. It was returned in the body of a direct
// TLS connection to Google's token endpoint, authenticated with this server's client
// secret. Nothing in between could have substituted it without breaking TLS, so the
// signature adds no information — this is the case OpenID Connect Core 3.1.3.7 calls
// out, where the client MAY skip id_token signature validation when the token is
// received directly from the token endpoint over a protected channel.
//
// THIS REASONING DOES NOT TRANSFER. An id_token arriving from a client — in a request
// body, a header, a redirect fragment — is attacker-controlled and MUST have its
// signature checked against Google's JWKS. If a future endpoint accepts one of those,
// it needs a real verifier, not this function.
//
// The claims that do not depend on the transport are still checked: aud must be this
// client (a token minted for a different application would otherwise be accepted),
// iss must be Google, and exp must be in the future.
func (provider *GoogleProvider) claimsFromIDToken(idToken string) (googleIDClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return googleIDClaims{}, errors.New("the id_token is not a three part JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return googleIDClaims{}, fmt.Errorf("decode the id_token payload: %w", err)
	}

	var claims googleIDClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return googleIDClaims{}, fmt.Errorf("decode the id_token claims: %w", err)
	}

	if claims.Audience != provider.clientID {
		return googleIDClaims{}, fmt.Errorf("the id_token was issued to %q, not to this client",
			claims.Audience)
	}
	// Google uses both forms, with and without the scheme prefix.
	if claims.Issuer != googleIssuer && claims.Issuer != "accounts.google.com" {
		return googleIDClaims{}, fmt.Errorf("unexpected id_token issuer %q", claims.Issuer)
	}
	if claims.Expiry <= 0 || provider.now().UTC().After(time.Unix(claims.Expiry, 0).UTC()) {
		return googleIDClaims{}, errors.New("the id_token has expired")
	}
	if claims.Subject == "" {
		return googleIDClaims{}, errors.New("the id_token carried no subject")
	}
	return claims, nil
}

// flexibleBool decodes a JSON true/false or "true"/"false".
//
// Google sends a bool. Accepting the string form as well costs nothing and means a
// provider that sends it does not silently fail decoding — which, for this particular
// field, would fail in the safe direction (unverified) but with a confusing error.
type flexibleBool bool

func (value *flexibleBool) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(string(data), `"`)
	switch trimmed {
	case "true":
		*value = true
	case "false", "", "null":
		*value = false
	default:
		return fmt.Errorf("auth: %q is not a boolean", trimmed)
	}
	return nil
}
