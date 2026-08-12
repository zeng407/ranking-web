package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Issuer mints the access tokens the API verifies.
//
// Byte-for-byte the same shape App\Services\Auth\GoAccessTokenService produces, and
// signed with the same key, so during the cutover a token from either side is
// accepted by the same Verifier. That is what lets the SPA's login move to Go
// without touching anything that consumes a token, and it is checked by a test that
// verifies a real PHP-issued token and a Go-issued one through one verifier.
type Issuer struct {
	issuer     string
	audience   string
	keyID      string
	privateKey ed25519.PrivateKey
	ttl        time.Duration
	now        func() time.Time
}

// IssuerConfig configures Issuer.
type IssuerConfig struct {
	Issuer   string
	Audience string
	// KeyID is the `kid` header. It has no effect on verification today — there is
	// one key — but it is what makes rotating that key possible later without
	// invalidating every live token.
	KeyID string
	// PrivateKey is a 32-byte seed or a 64-byte Ed25519 private key, matching what
	// GoAccessTokenService accepts for GO_AUTH_PRIVATE_KEY.
	PrivateKey ed25519.PrivateKey
	TTL        time.Duration
	Now        func() time.Time
}

// AccessTokenTTLBounds mirror the range GoAccessTokenService enforces. Short
// because an access token cannot be revoked: the only thing limiting the damage
// from a stolen one is how quickly it expires.
const (
	MinAccessTokenTTL     = 60 * time.Second
	MaxAccessTokenTTL     = 900 * time.Second
	DefaultAccessTokenTTL = 300 * time.Second
)

func NewIssuer(config IssuerConfig) (*Issuer, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("auth: issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("auth: audience is required")
	}
	if len(config.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("auth: private key must be %d bytes, got %d",
			ed25519.PrivateKeySize, len(config.PrivateKey))
	}
	if config.TTL == 0 {
		config.TTL = DefaultAccessTokenTTL
	}
	if config.TTL < MinAccessTokenTTL || config.TTL > MaxAccessTokenTTL {
		return nil, fmt.Errorf("auth: access token ttl must be between %s and %s, got %s",
			MinAccessTokenTTL, MaxAccessTokenTTL, config.TTL)
	}
	keyID := strings.TrimSpace(config.KeyID)
	if keyID == "" {
		keyID = "primary"
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Issuer{
		issuer:     config.Issuer,
		audience:   config.Audience,
		keyID:      keyID,
		privateKey: append(ed25519.PrivateKey(nil), config.PrivateKey...),
		ttl:        config.TTL,
		now:        now,
	}, nil
}

// PrivateKeyFromBase64 decodes GO_AUTH_PRIVATE_KEY.
//
// Accepts both forms GoAccessTokenService does: a 32-byte seed, or a full 64-byte
// private key. Anything else is rejected rather than padded, because a key that is
// silently the wrong length would produce tokens no verifier accepts.
func PrivateKeyFromBase64(encoded string) (ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, errors.New("auth: private key is empty")
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("auth: private key must be valid base64: %w", err)
		}
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf("auth: private key must be a %d-byte seed or a %d-byte key, got %d bytes",
			ed25519.SeedSize, ed25519.PrivateKeySize, len(raw))
	}
}

// AccessToken is a signed token and the moment it stops being valid.
type AccessToken struct {
	Token     string    `json:"access_token"`
	TokenType string    `json:"token_type"`
	ExpiresIn int       `json:"expires_in"`
	ExpiresAt time.Time `json:"-"`
}

// Issue signs an access token for a user.
//
// roles carries every slug the user holds, including "banned" — the same set
// GoAccessTokenService puts in the claim. Filtering here would hide the state from
// every consumer; whether a banned user may act is a decision for the endpoint that
// reads the claim, not for the thing that describes who they are.
func (issuer *Issuer) Issue(userID int64, roles []string) (AccessToken, error) {
	if userID <= 0 {
		return AccessToken{}, fmt.Errorf("auth: cannot issue a token for user id %d", userID)
	}

	now := issuer.now().UTC()
	expiresAt := now.Add(issuer.ttl)

	tokenID, err := randomHex(16)
	if err != nil {
		return AccessToken{}, err
	}

	// An absent roles list must encode as [] rather than null: the verifier copies
	// the slice, and a consumer iterating null would be a different shape from what
	// the PHP produces.
	if roles == nil {
		roles = []string{}
	}

	header := tokenHeader{Algorithm: "EdDSA", Type: "at+jwt", KeyID: issuer.keyID}
	claims := tokenClaims{
		Issuer:    issuer.issuer,
		Audience:  issuer.audience,
		Subject:   fmt.Sprintf("%d", userID),
		Roles:     roles,
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		TokenID:   tokenID,
	}

	encodedHeader, err := encodeSegment(header)
	if err != nil {
		return AccessToken{}, err
	}
	encodedClaims, err := encodeSegment(claims)
	if err != nil {
		return AccessToken{}, err
	}

	signingInput := encodedHeader + "." + encodedClaims
	signature := ed25519.Sign(issuer.privateKey, []byte(signingInput))

	return AccessToken{
		Token:     signingInput + "." + base64.RawURLEncoding.EncodeToString(signature),
		TokenType: "Bearer",
		ExpiresIn: int(issuer.ttl.Seconds()),
		ExpiresAt: expiresAt,
	}, nil
}

func encodeSegment(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("auth: encode token segment: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

// randomHex returns n cryptographically random bytes, hex encoded.
func randomHex(n int) (string, error) {
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
