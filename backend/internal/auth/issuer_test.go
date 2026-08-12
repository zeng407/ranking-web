package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func generateTestKeypair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	public, private, err := ed25519.GenerateKey(nil)
	return private, public, err
}

func publicKeyFromBase64(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(raw), nil
}

func testIssuerAndVerifier(t *testing.T, now time.Time) (*Issuer, *Verifier) {
	t.Helper()
	private, public, err := generateTestKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "2pick-go-api",
		PrivateKey: private, TTL: DefaultAccessTokenTTL, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	verifier, err := NewVerifier(Config{
		Issuer: "http://localhost", Audience: "2pick-go-api", PublicKey: public,
		MaxTokenLifetime: MaxAccessTokenTTL, ClockSkew: 30 * time.Second,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	return issuer, verifier
}

func TestIssuedTokensVerify(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	issuer, verifier := testIssuerAndVerifier(t, now)

	issued, err := issuer.Issue(99, []string{"admin", "banned"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.TokenType != "Bearer" {
		t.Errorf("token type = %q, want Bearer", issued.TokenType)
	}
	if issued.ExpiresIn != int(DefaultAccessTokenTTL.Seconds()) {
		t.Errorf("expires_in = %d, want %d", issued.ExpiresIn, int(DefaultAccessTokenTTL.Seconds()))
	}

	identity, err := verifier.Verify(issued.Token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if identity.Subject != "99" {
		t.Errorf("subject = %q, want 99", identity.Subject)
	}
	// Every slug is carried, "banned" included: filtering it here would hide the
	// state from the endpoints that have to act on it.
	if len(identity.Roles) != 2 || identity.Roles[1] != "banned" {
		t.Errorf("roles = %v, want [admin banned]", identity.Roles)
	}
}

// Two tokens for the same user must differ, or a leaked token could not be told
// apart from a fresh one in a log and jti would be useless.
func TestEachTokenIsUnique(t *testing.T) {
	now := time.Now().UTC()
	issuer, _ := testIssuerAndVerifier(t, now)

	first, err := issuer.Issue(1, nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	second, err := issuer.Issue(1, nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if first.Token == second.Token {
		t.Fatal("two tokens issued for the same user at the same instant are identical")
	}
}

func TestExpiredTokensAreRejected(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	issuer, _ := testIssuerAndVerifier(t, now)
	issued, err := issuer.Issue(7, nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	// A verifier whose clock is past the expiry, beyond the skew allowance.
	_, public, _ := generateTestKeypair()
	_ = public
	later := now.Add(DefaultAccessTokenTTL + time.Minute)
	_, verifier := testIssuerAndVerifier(t, later)
	if _, err := verifier.Verify(issued.Token); err == nil {
		t.Fatal("an expired token must not verify")
	}
}

func TestIssuerRejectsBadConfiguration(t *testing.T) {
	private, _, _ := generateTestKeypair()
	base := IssuerConfig{Issuer: "http://localhost", Audience: "2pick-go-api", PrivateKey: private}

	for name, mutate := range map[string]func(*IssuerConfig){
		"no issuer":   func(c *IssuerConfig) { c.Issuer = "" },
		"no audience": func(c *IssuerConfig) { c.Audience = "" },
		"no key":      func(c *IssuerConfig) { c.PrivateKey = nil },
		"short key":   func(c *IssuerConfig) { c.PrivateKey = private[:16] },
		// Outside the range GoAccessTokenService enforces. An access token cannot be
		// revoked, so a long one is a standing risk.
		"ttl too long":  func(c *IssuerConfig) { c.TTL = time.Hour },
		"ttl too short": func(c *IssuerConfig) { c.TTL = time.Second },
	} {
		config := base
		mutate(&config)
		if _, err := NewIssuer(config); err == nil {
			t.Errorf("NewIssuer() should reject the %q case", name)
		}
	}
	if _, err := NewIssuer(base); err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
}

func TestIssueRejectsABadUserID(t *testing.T) {
	issuer, _ := testIssuerAndVerifier(t, time.Now())
	for _, id := range []int64{0, -1} {
		if _, err := issuer.Issue(id, nil); err == nil {
			t.Errorf("Issue(%d) should be rejected", id)
		}
	}
}

// GO_AUTH_PRIVATE_KEY accepts both forms GoAccessTokenService does.
func TestPrivateKeyAcceptsSeedAndFullKey(t *testing.T) {
	private, _, _ := generateTestKeypair()
	seed := private.Seed()

	fromSeed, err := PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatalf("seed form rejected: %v", err)
	}
	fromFull, err := PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(private))
	if err != nil {
		t.Fatalf("full-key form rejected: %v", err)
	}
	if !fromSeed.Equal(fromFull) {
		t.Fatal("the seed and the full key produced different private keys")
	}

	for name, encoded := range map[string]string{
		"empty":        "",
		"not base64":   "!!!!",
		"wrong length": base64.StdEncoding.EncodeToString([]byte("too short")),
	} {
		if _, err := PrivateKeyFromBase64(encoded); err == nil {
			t.Errorf("PrivateKeyFromBase64 should reject the %q case", name)
		}
	}
}
