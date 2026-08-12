package auth

import (
	"os"
	"strings"
	"testing"
	"time"
)

// interopFixture carries a real Ed25519 keypair and a token that PHP actually
// signed with it, produced by the same code path as
// App\Services\Auth\GoAccessTokenService.
//
// Supplied through the environment rather than committed because it is a private
// key; the test skips without it so the hermetic build is unaffected.
func interopFixture(t *testing.T) (privateB64, publicB64, phpToken string) {
	t.Helper()
	privateB64 = os.Getenv("AUTH_INTEROP_PRIVATE")
	publicB64 = os.Getenv("AUTH_INTEROP_PUBLIC")
	phpToken = os.Getenv("AUTH_INTEROP_PHP_TOKEN")
	if privateB64 == "" || publicB64 == "" || phpToken == "" {
		t.Skip("AUTH_INTEROP_* are not set; skipping the PHP interop test")
	}
	return
}

func interopVerifier(t *testing.T, publicB64 string, now time.Time) *Verifier {
	t.Helper()
	key, err := publicKeyFromBase64(publicB64)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	verifier, err := NewVerifier(Config{
		Issuer:           "http://localhost",
		Audience:         "2pick-go-api",
		PublicKey:        key,
		MaxTokenLifetime: MaxAccessTokenTTL,
		ClockSkew:        30 * time.Second,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	return verifier
}

// THE POINT OF THE CUTOVER. A token PHP signed and a token Go signed have to be
// accepted by the same verifier, or moving login to Go would invalidate every
// session the moment it shipped.
func TestGoAndPHPTokensAreInterchangeable(t *testing.T) {
	privateB64, publicB64, phpToken := interopFixture(t)
	now := time.Now().UTC()
	verifier := interopVerifier(t, publicB64, now)

	// The PHP-issued token, verified by the Go verifier.
	phpIdentity, err := verifier.Verify(phpToken)
	if err != nil {
		t.Fatalf("the verifier rejected a real PHP-issued token: %v", err)
	}
	if phpIdentity.Subject != "4242" {
		t.Errorf("php subject = %q, want 4242", phpIdentity.Subject)
	}
	if len(phpIdentity.Roles) != 1 || phpIdentity.Roles[0] != "admin" {
		t.Errorf("php roles = %v, want [admin]", phpIdentity.Roles)
	}

	// A Go-issued token for the same user, signed with the same key.
	privateKey, err := PrivateKeyFromBase64(privateB64)
	if err != nil {
		t.Fatalf("PrivateKeyFromBase64() error = %v", err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		Issuer:     "http://localhost",
		Audience:   "2pick-go-api",
		PrivateKey: privateKey,
		TTL:        DefaultAccessTokenTTL,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	issued, err := issuer.Issue(4242, []string{"admin"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	goIdentity, err := verifier.Verify(issued.Token)
	if err != nil {
		t.Fatalf("the verifier rejected a Go-issued token: %v", err)
	}
	if goIdentity.Subject != phpIdentity.Subject {
		t.Errorf("subjects differ: go %q, php %q", goIdentity.Subject, phpIdentity.Subject)
	}
	if len(goIdentity.Roles) != len(phpIdentity.Roles) || goIdentity.Roles[0] != phpIdentity.Roles[0] {
		t.Errorf("roles differ: go %v, php %v", goIdentity.Roles, phpIdentity.Roles)
	}

	// And the wire shape matches: same header, same claim names.
	phpHeader := strings.Split(phpToken, ".")[0]
	goHeader := strings.Split(issued.Token, ".")[0]
	if phpHeader != goHeader {
		t.Errorf("headers differ:\n  php %s\n  go  %s", phpHeader, goHeader)
	}
}

// A key from the wrong pair must not verify, or the interop test above would pass
// for the wrong reason.
func TestAForeignKeyIsRejected(t *testing.T) {
	_, publicB64, _ := interopFixture(t)
	now := time.Now().UTC()

	other, _, err := generateTestKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "2pick-go-api",
		PrivateKey: other, TTL: DefaultAccessTokenTTL, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	issued, err := issuer.Issue(4242, nil)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := interopVerifier(t, publicB64, now).Verify(issued.Token); err == nil {
		t.Fatal("a token signed by a different key must not verify")
	}
}
