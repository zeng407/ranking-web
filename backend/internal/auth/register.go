package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Registration limits, taken from config/setting.php rather than from the column widths.
//
// MaxEmailForRegistration is 50 while the column is VARCHAR(255) and MaxEmailBytes (what
// the login path accepts) is 255. That is not an inconsistency to tidy up: the validator
// has always been the narrower rule, so there may be existing accounts with longer
// addresses that must still be able to log in. Registration keeps the old limit; login
// keeps accepting what is already stored.
const (
	MaxNameRunes            = 20
	MaxEmailForRegistration = 50
	MinPasswordRunes        = 8
	// BcryptCost matches config/hashing.php's rounds. A hash written at a different
	// cost still verifies, but writing a cheaper one would quietly weaken every new
	// account.
	BcryptCost = 10
)

// FieldErrors maps a form field to machine-readable reasons.
//
// Codes, not sentences. The Go API has no message catalogue and should not grow one:
// the SPA already translates into three languages, and a sentence chosen here could
// only ever be in one of them.
type FieldErrors map[string][]string

// Validation codes.
const (
	CodeRequired     = "required"
	CodeTooLong      = "too_long"
	CodeTooShort     = "too_short"
	CodeInvalidEmail = "invalid_email"
	CodeTaken        = "taken"
	CodeMismatch     = "mismatch"
)

// ErrRegistrationInvalid carries the per-field reasons a registration was refused.
type ErrRegistrationInvalid struct {
	Fields FieldErrors
}

func (err *ErrRegistrationInvalid) Error() string {
	return fmt.Sprintf("auth: registration is not valid: %v", err.Fields)
}

// Registration is one sign-up attempt, as the form submitted it.
type Registration struct {
	Name                 string
	Email                string
	Password             string
	PasswordConfirmation string
}

// UserWriter creates password accounts.
type UserWriter interface {
	// EmailExists reports whether the address is taken.
	EmailExists(ctx context.Context, email string) (bool, error)
	// CreateUser writes the account. It must return ErrOAuthEmailTaken when the unique
	// index on the address rejects the insert, so a race with another sign-up is
	// reported as a field error rather than a 500.
	CreateUser(ctx context.Context, record NewUser) (Credentials, error)
}

// NewUser is a password account.
type NewUser struct {
	Name         string
	Email        string
	PasswordHash string
}

// Register creates a password account and signs it in.
//
// Signing in as part of registering matches Laravel's RegistersUsers trait, which logs
// the user in on success. Making them log in again immediately would be a change in
// behaviour for no benefit.
func (service *Service) Register(
	ctx context.Context, registration Registration, client ClientInfo,
) (Grant, error) {
	if service.registrations == nil {
		return Grant{}, errors.New("auth: registration is not configured")
	}

	name := strings.TrimSpace(registration.Name)
	email := strings.TrimSpace(registration.Email)
	fields := FieldErrors{}

	switch {
	case name == "":
		fields["name"] = []string{CodeRequired}
	case utf8.RuneCountInString(name) > MaxNameRunes:
		// Runes, not bytes: the column is utf8mb4 and these names are mostly Chinese.
		fields["name"] = []string{CodeTooLong}
	}

	switch {
	case email == "":
		fields["email"] = []string{CodeRequired}
	case utf8.RuneCountInString(email) > MaxEmailForRegistration:
		fields["email"] = []string{CodeTooLong}
	case !looksLikeEmail(email):
		fields["email"] = []string{CodeInvalidEmail}
	}

	switch {
	case registration.Password == "":
		fields["password"] = []string{CodeRequired}
	case utf8.RuneCountInString(registration.Password) < MinPasswordRunes:
		fields["password"] = []string{CodeTooShort}
	case len(registration.Password) > MaxPasswordBytes:
		// bcrypt silently truncates past 72 bytes, so a longer password is not more
		// secure — and accepting it would mean the stored hash does not cover what the
		// user typed.
		fields["password"] = []string{CodeTooLong}
	case registration.Password != registration.PasswordConfirmation:
		fields["password_confirmation"] = []string{CodeMismatch}
	}

	// The address check runs only when the address is otherwise valid, so a malformed
	// one does not cost a query.
	if len(fields) == 0 {
		taken, err := service.registrations.EmailExists(ctx, email)
		if err != nil {
			return Grant{}, err
		}
		if taken {
			fields["email"] = []string{CodeTaken}
		}
	}

	if len(fields) > 0 {
		return Grant{}, &ErrRegistrationInvalid{Fields: fields}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(registration.Password), BcryptCost)
	if err != nil {
		return Grant{}, fmt.Errorf("auth: hash password: %w", err)
	}

	credentials, err := service.registrations.CreateUser(ctx, NewUser{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	})
	if errors.Is(err, ErrOAuthEmailTaken) {
		// Two sign-ups for the same address at once; the unique index arbitrated. The
		// answer the caller wants is the one the check above would have given.
		return Grant{}, &ErrRegistrationInvalid{Fields: FieldErrors{"email": {CodeTaken}}}
	}
	if err != nil {
		return Grant{}, err
	}

	service.logger.Info("account_registered", "user_id", credentials.UserID)
	return service.grant(ctx, credentials, client)
}

// looksLikeEmail is a deliberately loose check.
//
// One @, something before it, and a dot-bearing host after it. Not RFC 5322: a stricter
// pattern rejects addresses that exist, and the only authority on whether an address
// works is whether mail to it arrives. Laravel's `email` rule is similarly permissive by
// default.
func looksLikeEmail(value string) bool {
	at := strings.IndexByte(value, '@')
	if at <= 0 || at != strings.LastIndexByte(value, '@') {
		return false
	}
	host := value[at+1:]
	if len(host) < 3 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	dot := strings.IndexByte(host, '.')
	return dot > 0 && dot < len(host)-1
}
