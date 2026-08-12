package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifierAcceptsLaravelCompatibleToken(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	verifier := mustVerifier(t, publicKey, now)
	token := signedToken(t, privateKey, tokenClaims{
		Issuer: "https://2pick.app", Audience: "2pick-go-api", Subject: "42",
		Roles: []string{"admin"}, IssuedAt: now.Unix(), NotBefore: now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(), TokenID: "token-1",
	})

	identity, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if identity.Subject != "42" || len(identity.Roles) != 1 || identity.Roles[0] != "admin" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestVerifierPreservesAnEmptyRoleList(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	verifier := mustVerifier(t, publicKey, now)
	token := signedToken(t, privateKey, tokenClaims{
		Issuer: "https://2pick.app", Audience: "2pick-go-api", Subject: "42", Roles: []string{},
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), TokenID: "token-1",
	})

	identity, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if identity.Roles == nil || len(identity.Roles) != 0 {
		t.Fatalf("Roles = %#v, want an empty non-nil list", identity.Roles)
	}
}

func TestVerifierRejectsInvalidSignatureAndClaims(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	verifier := mustVerifier(t, publicKey, now)
	validClaims := tokenClaims{
		Issuer: "https://2pick.app", Audience: "2pick-go-api", Subject: "42",
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), TokenID: "token-1",
	}

	tests := map[string]string{
		"malformed":         "not-a-token",
		"invalid signature": signedToken(t, privateKey, validClaims) + "changed",
		"wrong audience": signedToken(t, privateKey, func() tokenClaims {
			claims := validClaims
			claims.Audience = "another-api"
			return claims
		}()),
		"expired": signedToken(t, privateKey, func() tokenClaims {
			claims := validClaims
			claims.IssuedAt = now.Add(-10 * time.Minute).Unix()
			claims.NotBefore = claims.IssuedAt
			claims.ExpiresAt = now.Add(-time.Minute).Unix()
			return claims
		}()),
		"excessive lifetime": signedToken(t, privateKey, func() tokenClaims {
			claims := validClaims
			claims.ExpiresAt = now.Add(20 * time.Minute).Unix()
			return claims
		}()),
	}

	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.Verify(token); err == nil {
				t.Fatal("Verify() should reject token")
			}
		})
	}
}

func mustVerifier(t *testing.T, publicKey ed25519.PublicKey, now time.Time) *Verifier {
	t.Helper()
	verifier, err := NewVerifier(Config{
		Issuer:           "https://2pick.app",
		Audience:         "2pick-go-api",
		PublicKey:        publicKey,
		MaxTokenLifetime: 10 * time.Minute,
		ClockSkew:        30 * time.Second,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	return verifier
}

func signedToken(t *testing.T, privateKey ed25519.PrivateKey, claims tokenClaims) string {
	t.Helper()
	headerJSON, err := json.Marshal(tokenHeader{Algorithm: "EdDSA", Type: "at+jwt", KeyID: "primary"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := header + "." + payload
	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}
