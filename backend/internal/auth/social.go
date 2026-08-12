package auth

import (
	"context"
	"time"
)

// SocialStore is the provider-link half of the account store.
//
// Separate from UserStore because the two answer different questions: UserStore is
// "who owns this address and what is their password", SocialStore is "which account
// does this provider subject belong to". Only the OAuth path needs the second.
type SocialStore interface {
	// FindByProviderSubject resolves a provider account to a local one. Must return
	// ErrUserNotFound when there is no link.
	FindByProviderSubject(ctx context.Context, provider, subject string) (Credentials, error)
	// EmailExists reports whether any account already holds the address.
	EmailExists(ctx context.Context, email string) (bool, error)
	// CreateLinkedUser creates the account and its provider link together. Must be
	// atomic: an account with no link cannot be signed into at all, since it has no
	// password either.
	CreateLinkedUser(ctx context.Context, record NewLinkedUser) (Credentials, error)
	// Link attaches a provider account to an existing user. Must return
	// ErrOAuthAlreadyLinked when either side is already taken.
	Link(ctx context.Context, request LinkRequest) error
}

// NewLinkedUser is an account created from a provider identity.
//
// There is no password field. These accounts have password = ” — the same empty
// string Laravel writes, not NULL, because the column is NOT NULL — and the login path
// refuses to compare against it. That is the 11,040-account guard in service.go.
type NewLinkedUser struct {
	Provider      string
	Subject       string
	Email         string
	Name          string
	AvatarURL     string
	EmailVerified bool
	CreatedAt     time.Time
}

// LinkRequest attaches a provider account to a user who already exists.
type LinkRequest struct {
	UserID   int64
	Provider string
	Subject  string
	Email    string
}
