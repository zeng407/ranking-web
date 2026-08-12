package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid access token")

type Config struct {
	Issuer           string
	Audience         string
	PublicKey        ed25519.PublicKey
	MaxTokenLifetime time.Duration
	ClockSkew        time.Duration
	Now              func() time.Time
}

type Identity struct {
	Subject   string
	Roles     []string
	ExpiresAt time.Time
}

type TokenVerifier interface {
	Verify(token string) (Identity, error)
}

type Verifier struct {
	issuer           string
	audience         string
	publicKey        ed25519.PublicKey
	maxTokenLifetime time.Duration
	clockSkew        time.Duration
	now              func() time.Time
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type tokenClaims struct {
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud"`
	Subject   string   `json:"sub"`
	Roles     []string `json:"roles"`
	IssuedAt  int64    `json:"iat"`
	NotBefore int64    `json:"nbf"`
	ExpiresAt int64    `json:"exp"`
	TokenID   string   `json:"jti"`
}

type contextKey string

const identityContextKey contextKey = "authenticated-identity"

func NewVerifier(config Config) (*Verifier, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("auth issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("auth audience is required")
	}
	if len(config.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("auth public key must be %d bytes", ed25519.PublicKeySize)
	}
	if config.MaxTokenLifetime <= 0 {
		return nil, errors.New("max token lifetime must be positive")
	}
	if config.ClockSkew < 0 {
		return nil, errors.New("clock skew cannot be negative")
	}
	if config.Now == nil {
		config.Now = time.Now
	}

	return &Verifier{
		issuer:           config.Issuer,
		audience:         config.Audience,
		publicKey:        append(ed25519.PublicKey(nil), config.PublicKey...),
		maxTokenLifetime: config.MaxTokenLifetime,
		clockSkew:        config.ClockSkew,
		now:              config.Now,
	}, nil
}

func (v *Verifier) Verify(token string) (Identity, error) {
	if len(token) > 8192 {
		return Identity{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Identity{}, ErrInvalidToken
	}

	var header tokenHeader
	if err := decodeSegment(parts[0], &header); err != nil {
		return Identity{}, ErrInvalidToken
	}
	if header.Algorithm != "EdDSA" || header.Type != "at+jwt" {
		return Identity{}, ErrInvalidToken
	}

	var claims tokenClaims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Identity{}, ErrInvalidToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Identity{}, ErrInvalidToken
	}
	if !ed25519.Verify(v.publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return Identity{}, ErrInvalidToken
	}

	if err := v.validateClaims(claims); err != nil {
		return Identity{}, ErrInvalidToken
	}

	roles := make([]string, len(claims.Roles))
	copy(roles, claims.Roles)

	return Identity{
		Subject:   claims.Subject,
		Roles:     roles,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
	}, nil
}

func (v *Verifier) validateClaims(claims tokenClaims) error {
	if claims.Issuer != v.issuer || claims.Audience != v.audience {
		return ErrInvalidToken
	}
	if claims.Subject == "" || claims.TokenID == "" {
		return ErrInvalidToken
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return ErrInvalidToken
	}
	if claims.IssuedAt <= 0 || claims.NotBefore <= 0 || claims.ExpiresAt <= claims.IssuedAt || claims.NotBefore >= claims.ExpiresAt {
		return ErrInvalidToken
	}
	maxLifetimeSeconds := int64(v.maxTokenLifetime / time.Second)
	if claims.ExpiresAt-claims.IssuedAt > maxLifetimeSeconds {
		return ErrInvalidToken
	}

	now := v.now()
	skew := v.clockSkew
	if now.Add(skew).Before(time.Unix(claims.NotBefore, 0)) {
		return ErrInvalidToken
	}
	if now.Add(skew).Before(time.Unix(claims.IssuedAt, 0)) {
		return ErrInvalidToken
	}
	if !now.Add(-skew).Before(time.Unix(claims.ExpiresAt, 0)) {
		return ErrInvalidToken
	}

	return nil
}

func decodeSegment(segment string, target any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON data")
	}
	return nil
}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey).(Identity)
	return identity, ok
}
