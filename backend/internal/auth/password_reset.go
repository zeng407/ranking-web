package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Forgot password and reset, from Auth\ForgotPasswordController and
// Auth\ResetPasswordController.
//
// This was the last thing only Laravel could do, because it is the only feature in the
// product that sends mail. It lives in this package for the same reason ChangePassword
// does: finishing a reset writes the hash, ends the account's other sessions and issues a
// new one, and only the thing that issues sessions can do that.

// Reset limits. Both match what Laravel's password broker was configured with, so the
// behaviour a user already knows does not change under them.
const (
	// ResetTokenTTL is config/auth.php passwords.users.expire, 60 minutes.
	ResetTokenTTL = time.Hour
	// ResetThrottle is passwords.users.throttle: one mail a minute per account.
	ResetThrottle = time.Minute
)

// ErrResetTokenInvalid covers every way a reset link can fail to work: never issued,
// expired, or already used.
//
// ONE ERROR FOR ALL THREE, ON PURPOSE. "This link expired" and "this link was already
// used" would both confirm that the token was real, which tells anyone submitting guesses
// that they guessed a live one.
var ErrResetTokenInvalid = errors.New("auth: password reset token is not valid")

// CodeInvalid is what the token field reports for all three cases above.
const CodeInvalid = "invalid"

// PasswordResetStore owns the go_password_resets rows.
type PasswordResetStore interface {
	// Create records a request. The token itself is never passed in — only its hash.
	Create(ctx context.Context, record NewPasswordReset) error
	// LastRequestedAt is the newest request for an account, for the throttle. The
	// boolean is false when the account has never asked.
	LastRequestedAt(ctx context.Context, userID int64) (time.Time, bool, error)
	// Consume claims a token for single use and answers whose account it belongs to.
	//
	// The claim has to be atomic — one statement that both checks and marks — or two
	// requests arriving together with the same token both read "unused" and both reset
	// the password. It must return ErrResetTokenInvalid when nothing was claimable.
	Consume(ctx context.Context, tokenHash string, at time.Time) (userID int64, err error)
}

// NewPasswordReset is one row of go_password_resets.
type NewPasswordReset struct {
	UserID      int64
	TokenHash   string
	RequestedAt time.Time
	ExpiresAt   time.Time
	// RequestedIP is audit only, like NewSession.CreatedIP.
	RequestedIP string
}

// ResetRequestLimiter caps how many reset mails one source may ask for.
//
// Optional, and separate from the per-account throttle, because the throttle cannot see
// the attack it does not cover: a script asking for one mail each for a thousand
// addresses is under the per-account limit every time. The mail leaves through one shared
// relay account, so the cost of not having this is that the relay gets locked and nobody
// can reset anything.
type ResetRequestLimiter interface {
	// AllowReset reports whether this source may ask for another mail.
	AllowReset(ctx context.Context, ip string) (bool, error)
}

// RequestPasswordReset mails a reset link.
//
// IT REPORTS SUCCESS FOR AN ADDRESS THAT HAS NO ACCOUNT, and for one that asked a moment
// ago, and for one whose mail the relay refused. Laravel did the same, rewriting every
// broker result except the throttle into the same "we have mailed you" response. The
// reason is account enumeration: a form that answers differently for a registered
// address is a way to test whether any given address has an account here, and this form
// needs no credentials to use.
//
// Validation errors are still reported — a malformed address cannot be a real account, so
// saying so leaks nothing and saves the user a wait for a mail that was never coming.
func (service *Service) RequestPasswordReset(
	ctx context.Context, email, locale string, client ClientInfo,
) error {
	if service.resets == nil || service.mail == nil || service.appURL == "" {
		return fmt.Errorf("%w: password reset", ErrNotConfigured)
	}

	email = strings.TrimSpace(email)
	switch {
	case email == "":
		return accountInvalid("email", CodeRequired)
	case len(email) > MaxEmailBytes:
		return accountInvalid("email", CodeTooLong)
	case !looksLikeEmail(email):
		return accountInvalid("email", CodeInvalidEmail)
	}

	if service.resetLimiter != nil && client.IP != "" {
		allowed, err := service.resetLimiter.AllowReset(ctx, client.IP)
		if err != nil {
			// The limiter is Redis, and Redis being unreachable must not take the
			// feature down with it. Logged, then treated as allowed: the per-account
			// throttle still applies.
			service.logger.ErrorContext(ctx, "password_reset_rate_limit_failed",
				"ip", client.IP, "error", err)
		} else if !allowed {
			service.logger.WarnContext(ctx, "password_reset_rate_limited", "ip", client.IP)
			return nil
		}
	}

	credentials, err := service.users.FindByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		// No row is written and no mail is sent, but the caller is told the same thing a
		// real account is told. See the doc comment.
		service.logger.InfoContext(ctx, "password_reset_requested_for_unknown_address")
		return nil
	}
	if err != nil {
		return err
	}

	now := service.now()
	lastRequested, found, err := service.resets.LastRequestedAt(ctx, credentials.UserID)
	if err != nil {
		return err
	}
	if found && now.Sub(lastRequested) < ResetThrottle {
		service.logger.InfoContext(ctx, "password_reset_throttled", "user_id", credentials.UserID)
		return nil
	}

	token, tokenHash, err := NewRefreshToken()
	if err != nil {
		return err
	}
	if err := service.resets.Create(ctx, NewPasswordReset{
		UserID:      credentials.UserID,
		TokenHash:   tokenHash,
		RequestedAt: now,
		ExpiresAt:   now.Add(ResetTokenTTL),
		RequestedIP: client.IP,
	}); err != nil {
		return err
	}

	message := passwordResetMessage(locale, email, service.appURL, token)
	if err := service.mail.Send(ctx, message); err != nil {
		// The row is already written, so the link works if the user has another copy —
		// and reporting a failure here would tell an unregistered address apart from a
		// registered one whose mail bounced. Loud in the log, because a relay that is
		// refusing mail is an operator problem.
		service.logger.ErrorContext(ctx, "password_reset_mail_failed",
			"user_id", credentials.UserID, "error", err)
		return nil
	}

	service.logger.InfoContext(ctx, "password_reset_mailed", "user_id", credentials.UserID)
	return nil
}

// ResetPassword finishes a reset and signs the account in.
//
// Signing in is not a convenience: the user has just proved control of the address on
// file, which is the same proof a login gives, and sending them to a login form to type
// the password they set two seconds ago is a step that only loses people.
//
// The account's other sessions end here, through the same path ChangePassword uses. A
// reset is what someone does when they think the password leaked, so leaving the sessions
// that password opened alive would defeat the exercise.
func (service *Service) ResetPassword(
	ctx context.Context, token, newPassword string, client ClientInfo,
) (Grant, error) {
	if service.resets == nil || service.accounts == nil {
		return Grant{}, fmt.Errorf("%w: password reset", ErrNotConfigured)
	}

	if strings.TrimSpace(token) == "" {
		return Grant{}, accountInvalid("token", CodeRequired)
	}
	// The password is checked before the token is spent, so a password the rules refuse
	// does not burn the link and force the user to ask for another one.
	if err := validateNewPassword(newPassword); err != nil {
		return Grant{}, err
	}

	userID, err := service.resets.Consume(ctx, HashToken(token), service.now())
	if errors.Is(err, ErrResetTokenInvalid) {
		return Grant{}, accountInvalid("token", CodeInvalid)
	}
	if err != nil {
		return Grant{}, err
	}

	credentials, err := service.users.FindByID(ctx, userID)
	if err != nil {
		// The token is spent by now. That is the right way round: a token that reached a
		// missing account must not be reusable.
		return Grant{}, err
	}

	service.logger.InfoContext(ctx, "password_reset_completed", "user_id", userID)
	return service.writePasswordAndRegrant(ctx, credentials, newPassword, client, true)
}

