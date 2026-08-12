// Package postaccess proves that a visitor knows a password-protected post's password.
//
// Laravel kept that proof in the server session: AccessTokenService wrote the post's own
// password hash under "{serial}_post_access_token", gave it thirty minutes, and refreshed
// it on every use. PostPolicy::newGame, PostPolicy::readRank and GamePolicy::play all read
// it back.
//
// This API has no session, so the proof travels with the caller as a signed token. It is
// deliberately NOT a JWT and not issued by auth.Issuer: an identity token and a
// "this browser knows post abcdefgh's door code" token must never be mistakable for one
// another, and auth.Issuer's tokens are keyed by user id, which a visitor here does not
// have.
package postaccess

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TTL matches AccessTokenService::TOKEN_EXPIRED_MINUTES.
const TTL = 30 * time.Minute

// keyLabel separates this key from anything else derived from the same secret.
const keyLabel = "2pick.post-access.v1"

var (
	// ErrInvalidToken covers every way a token fails: malformed, wrong post, expired, or
	// forged. The caller must not tell them apart — each distinction is a hint to someone
	// probing.
	ErrInvalidToken = errors.New("postaccess: token is not valid")
	// ErrWrongPassword means the password did not match the post's.
	ErrWrongPassword = errors.New("postaccess: wrong password")
)

// Signer issues and verifies post access tokens.
type Signer struct {
	key []byte
	now func() time.Time
}

// NewSigner derives the signing key from a secret the deployment already holds.
//
// Derived rather than used directly, so a leak of one cannot be replayed against the
// other: this key signs "knows a door code", the auth key signs "is user 42".
func NewSigner(secret []byte) (*Signer, error) {
	if len(secret) == 0 {
		return nil, errors.New("postaccess: a signing secret is required")
	}
	sum := sha256.Sum256(append([]byte(keyLabel+"\x00"), secret...))
	return &Signer{key: sum[:], now: time.Now}, nil
}

// Issue mints a token proving the holder knows serial's password.
//
// The token carries only the expiry and a signature. The serial is NOT in it: it is
// supplied by the request path at verification, so a token minted for one post cannot be
// presented for another — the MAC would not match.
func (signer *Signer) Issue(serial string) (token string, expiresAt time.Time) {
	expiresAt = signer.now().Add(TTL).Truncate(time.Second)
	stamp := strconv.FormatInt(expiresAt.Unix(), 10)
	return stamp + "." + base64.RawURLEncoding.EncodeToString(signer.sign(serial, stamp)), expiresAt
}

// Verify reports whether token proves knowledge of serial's password.
func (signer *Signer) Verify(serial, token string) error {
	stamp, mac, found := strings.Cut(strings.TrimSpace(token), ".")
	if !found || stamp == "" || mac == "" {
		return ErrInvalidToken
	}
	expiry, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return ErrInvalidToken
	}

	presented, err := base64.RawURLEncoding.DecodeString(mac)
	if err != nil {
		return ErrInvalidToken
	}
	// Compared before the expiry is judged, and with a constant-time compare, so neither
	// the answer nor the time taken tells a forger which half they got wrong.
	if !hmac.Equal(presented, signer.sign(serial, stamp)) {
		return ErrInvalidToken
	}
	if signer.now().After(time.Unix(expiry, 0)) {
		return ErrInvalidToken
	}
	return nil
}

func (signer *Signer) sign(serial, stamp string) []byte {
	mac := hmac.New(sha256.New, signer.key)
	// Length-prefixed, so a serial ending in digits cannot be shifted into the stamp to
	// produce the same input from different parts.
	fmt.Fprintf(mac, "%d:%s|%s", len(serial), serial, stamp)
	return mac.Sum(nil)
}

// HashPassword hashes a post password the way the column holds it.
//
// SHA-256, matching PostPolicy and the editor: the same digest that
// internal/authoring writes when an author sets the door code. Not bcrypt — this is a
// shared code handed out with the link, checked on every visitor's request, not a
// credential belonging to one person.
func HashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", sum)
}

// PasswordMatches compares a submitted password against the stored digest.
func PasswordMatches(password, storedDigest string) bool {
	if storedDigest == "" {
		return false
	}
	return hmac.Equal([]byte(HashPassword(password)), []byte(strings.ToLower(storedDigest)))
}
