package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// The account settings feature: the name, the avatar and the password, from
// Profile\ProfileController.
//
// It lives in this package rather than one of its own because two of the three
// operations are identity operations — changing a password has to be able to end the
// account's other sessions and re-issue the caller's own, which needs the refresh store
// and the issuer that Service already holds.

// Account is the settings view of an account.
//
// Wider than Profile deliberately. Profile answers /auth/me and is drawn on every page,
// so it carries the avatar and the name and nothing else. This is read by one page that
// the account holder opened on purpose, which is the one place the address belongs.
type Account struct {
	Name      string
	Email     string
	AvatarURL string
	// HasPassword is false for the 11,040 production accounts whose password column is
	// an empty string. They can call SetInitialPassword; everyone else must use
	// ChangePassword and prove the current one.
	HasPassword bool
	// GoogleLinked reports whether a user_socialities row exists.
	GoogleLinked bool
	// NameChangedAt is when the name was last changed, or the zero time for the 12,599
	// accounts that have never changed it.
	NameChangedAt time.Time
}

// AccountStore reads and writes the fields the settings page owns.
type AccountStore interface {
	// Account reads one account. It must return ErrUserNotFound when nothing matches.
	Account(ctx context.Context, userID int64) (Account, error)
	// UpdateName writes a new name, stamping name_updated_at, but only when
	// notChangedSince is later than the stored stamp. It reports whether the write
	// happened; false means the rate limit refused it.
	//
	// The guard belongs in the statement rather than in the caller: read-then-write
	// lets two requests in the same instant both see an old stamp and both pass.
	UpdateName(ctx context.Context, userID int64, name string,
		notChangedSince, changedAt time.Time) (bool, error)
	// UpdateAvatarURL points the account at an already-uploaded image.
	UpdateAvatarURL(ctx context.Context, userID int64, url string) error
	// UpdatePasswordHash replaces the bcrypt hash.
	UpdatePasswordHash(ctx context.Context, userID int64, hash string) error
}

// AvatarStore stores the uploaded image and answers with its public URL.
//
// Declared here rather than imported from internal/media so this package keeps knowing
// nothing about object storage; media.S3Store satisfies it as it stands.
type AvatarStore interface {
	Put(ctx context.Context, key string, body []byte, contentType string) (string, error)
}

// Account limits, from config/setting.php.
const (
	// MaxAvatarBytes is avatar_max_size. Laravel declared it and never applied it —
	// the rule was attached to a field named avatar_url while the file arrives as
	// avatar, so `sometimes` skipped it and any file of any type was accepted and
	// served. Here it is enforced, along with the image check.
	MaxAvatarBytes = 4 * 1024 * 1024
	// NameChangeDays is name_change_duration: a name may change once per this many
	// days. At 1 that means once per calendar day.
	NameChangeDays = 1
)

// Validation codes specific to the account settings.
const (
	CodeNameChangeTooSoon  = "name_change_too_soon"
	CodePasswordAlreadySet = "password_already_set"
	CodeNoPasswordSet      = "no_password_set"
	CodeIncorrect          = "incorrect"
	CodeUnsupportedImage   = "unsupported_image"
	CodeTooLarge           = "too_large"
)

// ErrNotConfigured means the process was started without the stores an operation needs.
//
// A distinct error rather than a generic one because it is not a fault: an api built
// without the object-store variables cannot accept avatars, and the caller has to be told
// "not here" (503) rather than "something broke" (500). The rest of this API already
// answers 503 with a *_not_configured code for exactly this.
var ErrNotConfigured = errors.New("auth: not configured")

// ErrAccountInvalid carries the per-field reasons a settings change was refused. Same
// shape as ErrRegistrationInvalid so the API renders both the same way.
type ErrAccountInvalid struct {
	Fields FieldErrors
}

func (err *ErrAccountInvalid) Error() string {
	return fmt.Sprintf("auth: account update rejected: %v", err.Fields)
}

func accountInvalid(field, code string) error {
	return &ErrAccountInvalid{Fields: FieldErrors{field: []string{code}}}
}

// Account reads the settings view of the signed-in account.
func (service *Service) Account(ctx context.Context, userID int64) (Account, error) {
	if service.accounts == nil {
		return Account{}, fmt.Errorf("%w: account settings", ErrNotConfigured)
	}
	return service.accounts.Account(ctx, userID)
}

// ChangeName renames the account.
//
// Returns the account as it now stands, so the caller does not have to read it back.
//
// UNCHANGED IS A SUCCESS, NOT A RATE-LIMIT REFUSAL. The original skipped the whole
// limit branch when the submitted name matched the stored one, and a settings form that
// posts every field would otherwise refuse a change of avatar because the name field
// still held the same value.
func (service *Service) ChangeName(ctx context.Context, userID int64, name string) (Account, error) {
	if service.accounts == nil {
		return Account{}, fmt.Errorf("%w: account settings", ErrNotConfigured)
	}

	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return Account{}, accountInvalid("name", CodeRequired)
	case utf8.RuneCountInString(name) > MaxNameRunes:
		return Account{}, accountInvalid("name", CodeTooLong)
	}

	account, err := service.accounts.Account(ctx, userID)
	if err != nil {
		return Account{}, err
	}
	if account.Name == name {
		return account, nil
	}

	now := service.now()
	written, err := service.accounts.UpdateName(ctx, userID, name, service.nameChangeBoundary(now), now)
	if err != nil {
		return Account{}, err
	}
	if !written {
		return Account{}, accountInvalid("name", CodeNameChangeTooSoon)
	}

	account.Name = name
	account.NameChangedAt = now
	return account, nil
}

// nameChangeBoundary is the newest name_updated_at that still allows a change.
//
// The original compared calendar dates — today().diffInDays(name_updated_at.toDateString())
// >= name_change_duration — so with a duration of one it means "not already changed
// today", not "not in the last 24 hours". Midnight is in the application's timezone,
// Asia/Taipei, because that is the day boundary the rule was written against.
func (service *Service) nameChangeBoundary(now time.Time) time.Time {
	local := now.In(service.location())
	startOfToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	return startOfToday.AddDate(0, 0, -(NameChangeDays - 1))
}

func (service *Service) location() *time.Location {
	if service.timezone != nil {
		return service.timezone
	}
	return time.UTC
}

// NameChangeAllowedAt is when the account may next change its name. It is in the past
// whenever a change is allowed now, which is what lets a client render the limit
// without reimplementing the rule.
func (service *Service) NameChangeAllowedAt(account Account) time.Time {
	if account.NameChangedAt.IsZero() {
		return time.Time{}
	}
	local := account.NameChangedAt.In(service.location())
	startOfDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	return startOfDay.AddDate(0, 0, NameChangeDays)
}

// UploadAvatar stores an image and points the account at it.
//
// The content type is sniffed from the bytes, never taken from the request: the browser
// sends whatever it likes, and this file is served back to every visitor who sees the
// account. The old object is left in place, as the original left it — deleting it needs
// the URL-to-key mapping and is worth doing on its own rather than inside a rename.
func (service *Service) UploadAvatar(
	ctx context.Context, userID int64, image []byte, keyName func(extension string) string,
) (string, error) {
	if service.accounts == nil || service.avatars == nil {
		return "", fmt.Errorf("%w: avatar uploads", ErrNotConfigured)
	}
	if len(image) == 0 {
		return "", accountInvalid("avatar", CodeRequired)
	}
	if len(image) > MaxAvatarBytes {
		return "", accountInvalid("avatar", CodeTooLarge)
	}

	contentType, extension, ok := imageKind(image)
	if !ok {
		return "", accountInvalid("avatar", CodeUnsupportedImage)
	}

	url, err := service.avatars.Put(ctx, keyName(extension), image, contentType)
	if err != nil {
		return "", fmt.Errorf("auth: store avatar: %w", err)
	}
	if err := service.accounts.UpdateAvatarURL(ctx, userID, url); err != nil {
		return "", err
	}
	return url, nil
}

// imageKind sniffs the format from the leading bytes.
//
// Only the four formats the site already serves. http.DetectContentType would answer
// for far more than that — including text/html for a file that merely starts with a
// tag, which is exactly the upload not to store under an image URL.
func imageKind(image []byte) (contentType, extension string, ok bool) {
	switch {
	case len(image) >= 8 && string(image[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png", "png", true
	case len(image) >= 3 && string(image[:3]) == "\xff\xd8\xff":
		return "image/jpeg", "jpg", true
	case len(image) >= 6 && (string(image[:6]) == "GIF87a" || string(image[:6]) == "GIF89a"):
		return "image/gif", "gif", true
	case len(image) >= 12 && string(image[:4]) == "RIFF" && string(image[8:12]) == "WEBP":
		return "image/webp", "webp", true
	}
	return "", "", false
}

// ChangePassword replaces the password after proving the current one, and issues a
// fresh session for the caller.
//
// EVERY OTHER SESSION ENDS. The reason to change a password is that the old one may be
// known to someone else, and a refresh token issued while it was is exactly what that
// someone would still be holding. The caller's own session is re-issued in the same
// call so the change does not sign them out of the page they are on.
//
// Laravel did neither, because its sessions were server-side and Laravel's own password
// rotation left them alone too. The store has carried RevokeUser "for a ban or a
// password change" since the refresh work; this is that second case.
func (service *Service) ChangePassword(
	ctx context.Context, userID int64, currentPassword, newPassword string, client ClientInfo,
) (Grant, error) {
	if service.accounts == nil {
		return Grant{}, fmt.Errorf("%w: account settings", ErrNotConfigured)
	}

	credentials, err := service.users.FindByID(ctx, userID)
	if err != nil {
		return Grant{}, err
	}
	if credentials.PasswordHash == "" {
		return Grant{}, accountInvalid("current_password", CodeNoPasswordSet)
	}
	if err := validateNewPassword(newPassword); err != nil {
		// Still pay the compare, so a rejected new password is not measurably faster
		// than a wrong current one.
		_ = bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(currentPassword))
		return Grant{}, err
	}
	if len(currentPassword) > MaxPasswordBytes {
		currentPassword = currentPassword[:MaxPasswordBytes]
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(credentials.PasswordHash), []byte(currentPassword)); err != nil {
		return Grant{}, accountInvalid("current_password", CodeIncorrect)
	}

	return service.writePasswordAndRegrant(ctx, credentials, newPassword, client, true)
}

// SetInitialPassword gives a password to an account that has none.
//
// The 11,040 accounts that signed in through Google hold an empty password column, and
// this is how they gain one without a reset mail. Refuses when a password is already
// set: that path has to prove the current one, or a stolen access token would be enough
// to take the account over.
//
// UNLIKE ChangePassword, THIS DOES NOT END THE OTHER SESSIONS. There was no password to
// have leaked, so signing the account out of its other devices would cost the user
// something and buy nothing.
func (service *Service) SetInitialPassword(
	ctx context.Context, userID int64, newPassword string, client ClientInfo,
) (Grant, error) {
	if service.accounts == nil {
		return Grant{}, fmt.Errorf("%w: account settings", ErrNotConfigured)
	}

	credentials, err := service.users.FindByID(ctx, userID)
	if err != nil {
		return Grant{}, err
	}
	if credentials.PasswordHash != "" {
		return Grant{}, accountInvalid("new_password", CodePasswordAlreadySet)
	}
	if err := validateNewPassword(newPassword); err != nil {
		return Grant{}, err
	}

	return service.writePasswordAndRegrant(ctx, credentials, newPassword, client, false)
}

func validateNewPassword(password string) error {
	switch {
	case password == "":
		return accountInvalid("new_password", CodeRequired)
	case utf8.RuneCountInString(password) < MinPasswordRunes:
		return accountInvalid("new_password", CodeTooShort)
	case len(password) > MaxPasswordBytes:
		// bcrypt truncates at 72 bytes, so accepting more would store a password the
		// user cannot reproduce the tail of.
		return accountInvalid("new_password", CodeTooLong)
	}
	return nil
}

func (service *Service) writePasswordAndRegrant(
	ctx context.Context, credentials Credentials, newPassword string,
	client ClientInfo, revokeOthers bool,
) (Grant, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), BcryptCost)
	if err != nil {
		return Grant{}, fmt.Errorf("auth: hash new password: %w", err)
	}
	if err := service.accounts.UpdatePasswordHash(ctx, credentials.UserID, string(hash)); err != nil {
		return Grant{}, err
	}

	if revokeOthers {
		// After the write, not before: revoking first and then failing to write would
		// have signed the account out for nothing.
		if revoked, err := service.sessions.RevokeUser(ctx, credentials.UserID, service.now()); err != nil {
			// The password is already changed, so refusing the request now would be a
			// lie. Report it and carry on: the sessions still expire on their own.
			service.logger.ErrorContext(ctx, "revoking sessions after a password change failed",
				"user_id", credentials.UserID, "error", err)
		} else {
			service.logger.InfoContext(ctx, "sessions revoked after a password change",
				"user_id", credentials.UserID, "sessions", revoked)
		}
	}

	return service.grant(ctx, credentials, client)
}
