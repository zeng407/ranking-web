package postaccess

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newSigner(t *testing.T) *Signer {
	t.Helper()
	signer, err := NewSigner([]byte("a-deployment-secret"))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}
	return signer
}

func TestATokenVerifiesForThePostItWasIssuedFor(t *testing.T) {
	signer := newSigner(t)

	token, expiresAt := signer.Issue("abcdefgh")

	if err := signer.Verify("abcdefgh", token); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	// Thirty minutes, matching AccessTokenService::TOKEN_EXPIRED_MINUTES.
	if got := time.Until(expiresAt); got > TTL || got < TTL-time.Minute {
		t.Errorf("expires in %v, want about %v", got, TTL)
	}
}

/*
A TOKEN FOR ONE POST MUST NOT OPEN ANOTHER.

The serial is not carried inside the token — it is supplied by the request path and signed
over. Presenting abcdefgh's token on ijklmnop therefore recomputes a different MAC and
fails, without the server having to remember which post the token was for.
*/
func TestATokenDoesNotOpenADifferentPost(t *testing.T) {
	signer := newSigner(t)

	token, _ := signer.Issue("abcdefgh")

	if err := signer.Verify("ijklmnop", token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	signer := newSigner(t)
	signer.now = func() time.Time { return time.Now().Add(-2 * TTL) }
	token, _ := signer.Issue("abcdefgh")

	signer.now = time.Now
	if err := signer.Verify("abcdefgh", token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

// A token whose expiry has been pushed forward must fail: the stamp is signed, so editing
// it breaks the MAC.
func TestExtendingTheExpiryByHandBreaksTheToken(t *testing.T) {
	signer := newSigner(t)
	token, _ := signer.Issue("abcdefgh")
	_, mac, _ := strings.Cut(token, ".")

	forged := strconv.Itoa(int(time.Now().Add(100*time.Hour).Unix())) + "." + mac

	if err := signer.Verify("abcdefgh", forged); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestARubbishTokenIsRefused(t *testing.T) {
	signer := newSigner(t)

	for _, token := range []string{
		"", "   ", "no-dot", ".", "abc.", ".abc",
		"notanumber.YWJj", "9999999999.!!!not-base64!!!",
	} {
		if err := signer.Verify("abcdefgh", token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%q: error = %v, want ErrInvalidToken", token, err)
		}
	}
}

// A different deployment secret must not verify: the key is derived from it.
func TestATokenFromAnotherSecretIsRefused(t *testing.T) {
	mine := newSigner(t)
	theirs, err := NewSigner([]byte("a-different-secret"))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	token, _ := theirs.Issue("abcdefgh")

	if err := mine.Verify("abcdefgh", token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

/*
THE LENGTH PREFIX EXISTS SO TWO DIFFERENT POSTS CANNOT SIGN THE SAME BYTES.

Without it, serial "ab" with stamp "1|c" and serial "ab|1" with stamp "c" would feed the
MAC identical input. Post serials are fixed-length today, so this is a guard against a
future where they are not, and it costs nothing.
*/
func TestSerialsThatCouldRunTogetherSignDifferently(t *testing.T) {
	signer := newSigner(t)

	first := signer.sign("ab", "1|c")
	second := signer.sign("ab|1", "c")

	if string(first) == string(second) {
		t.Error("two different (serial, stamp) pairs produced the same signature")
	}
}

func TestHashPasswordMatchesTheColumnsDigest(t *testing.T) {
	got := HashPassword("door-code")

	// Hex SHA-256, which is what PHP's hash('sha256', $password) writes and what
	// internal/authoring writes when an author sets the door code.
	if len(got) != 64 {
		t.Fatalf("digest = %q, want 64 hex characters", got)
	}
	if !PasswordMatches("door-code", got) {
		t.Error("a password does not match its own digest")
	}
	if PasswordMatches("not-the-code", got) {
		t.Error("a wrong password matched")
	}
}

// An empty stored digest is what a post with no password holds. Nothing may match it —
// least of all the empty string.
func TestNothingMatchesAPostWithNoPassword(t *testing.T) {
	if PasswordMatches("", "") {
		t.Error("the empty password matched a post with none set")
	}
	if PasswordMatches("anything", "") {
		t.Error("a password matched a post with none set")
	}
}

func TestNewSignerRequiresASecret(t *testing.T) {
	if _, err := NewSigner(nil); err == nil {
		t.Error("NewSigner() accepted an empty secret")
	}
}
