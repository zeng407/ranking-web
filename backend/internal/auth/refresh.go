package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Refresh token errors. They are deliberately indistinguishable to a caller that
// only inspects the HTTP status: an attacker probing with stolen tokens must not
// learn from the response whether a token exists, has expired, or was revoked.
var (
	// ErrRefreshTokenInvalid covers unknown, malformed, expired and revoked tokens.
	ErrRefreshTokenInvalid = errors.New("auth: refresh token is not valid")
	// ErrRefreshTokenReused is returned after a used token is presented again. It is
	// separate from the above only so the server can log the theft signal and revoke
	// the family; the response to the client is the same.
	ErrRefreshTokenReused = errors.New("auth: refresh token was already used")
	// ErrCSRFMismatch means the cookie was present but the echoed value was not.
	ErrCSRFMismatch = errors.New("auth: csrf token does not match")
)

// RefreshTokenTTL is how long a session survives without being used.
//
// Thirty days matches the "remember me" expectation of the Laravel session it
// replaces. Every refresh issues a fresh window, so an active user is never logged
// out; an abandoned session dies on its own.
const RefreshTokenTTL = 30 * 24 * time.Hour

// RefreshTokenBytes is the entropy in an opaque refresh token. 32 bytes is the
// reason a plain SHA-256 is enough to store it: there is nothing to brute force.
const RefreshTokenBytes = 32

// Session is one row of the rotation chain.
type Session struct {
	ID        int64
	UserID    int64
	FamilyID  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CSRFHash  string
}

// Used reports whether this token has already been exchanged.
func (session Session) Used() bool { return session.UsedAt != nil }

// Revoked reports whether the session was ended.
func (session Session) Revoked() bool { return session.RevokedAt != nil }

// Active reports whether the token may still be exchanged.
func (session Session) Active(now time.Time) bool {
	return !session.Used() && !session.Revoked() && now.Before(session.ExpiresAt)
}

// RefreshStore persists the rotation chain.
type RefreshStore interface {
	// Create records a newly issued token.
	Create(ctx context.Context, record NewSession) (Session, error)
	// FindByTokenHash looks up a presented token. It must return
	// ErrRefreshTokenInvalid when nothing matches.
	FindByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	// MarkUsed consumes a token as part of a rotation.
	MarkUsed(ctx context.Context, sessionID int64, usedAt time.Time) error
	// RevokeFamily ends every session in a rotation chain, which is the response to
	// a replayed token and to an explicit logout.
	RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) (int64, error)
	// RevokeUser ends every session a user holds, for a ban or a password change.
	RevokeUser(ctx context.Context, userID int64, revokedAt time.Time) (int64, error)
	// DeleteExpired purges rows past their expiry, for a scheduled cleanup.
	DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}

// NewSession is what Create writes.
type NewSession struct {
	UserID    int64
	FamilyID  string
	TokenHash string
	CSRFHash  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	CreatedIP string
	UserAgent string
}

// IssuedRefresh is what the caller hands to the client: the opaque token for the
// httpOnly cookie, and the CSRF value the client must echo back.
type IssuedRefresh struct {
	// Token goes in the httpOnly cookie. It is never readable by scripts, which is
	// what makes XSS unable to steal the session outright.
	Token string
	// CSRFToken is deliberately NOT httpOnly. The client has to read it to echo it
	// in a header, and that asymmetry is the whole mechanism: a cross-site request
	// carries the cookie automatically but cannot read this value to echo it.
	CSRFToken string
	FamilyID  string
	ExpiresAt time.Time
}

// NewRefreshToken generates an opaque token and its storage hash.
func NewRefreshToken() (token, hash string, err error) {
	raw := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken is how a presented token is turned into a lookup key.
//
// SHA-256 rather than bcrypt on purpose: the input is 32 random bytes, so there is
// no dictionary to defend against, and a slow hash on the refresh path would cost
// every legitimate request without making any attack harder.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewCSRFToken generates the value the client echoes back.
func NewCSRFToken() (token, hash string, err error) {
	raw := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// CheckCSRF compares a presented CSRF value against the stored hash.
//
// Constant time. The comparison is of hashes rather than raw values, but a
// byte-by-byte compare would still leak how much of a guess was correct, and this
// runs on a path an attacker can call freely.
func CheckCSRF(presented, storedHash string) error {
	if presented == "" || storedHash == "" {
		return ErrCSRFMismatch
	}
	if subtle.ConstantTimeCompare([]byte(HashToken(presented)), []byte(storedHash)) != 1 {
		return ErrCSRFMismatch
	}
	return nil
}

// NewFamilyID identifies one login's rotation chain.
func NewFamilyID() (string, error) {
	// A random 16-byte value formatted as a UUID. Not a real v4 — nothing parses it
	// as one — but it fits CHAR(36) and reads like an id in a log.
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
