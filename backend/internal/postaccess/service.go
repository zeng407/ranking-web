package postaccess

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Access policies, matching post_policies.access_policy.
const (
	PolicyPrivate  = "private"
	PolicyPublic   = "public"
	PolicyPassword = "password"
)

// ErrPostNotFound means no live post has that serial.
var ErrPostNotFound = errors.New("postaccess: post not found")

// ErrRateLimited means too many attempts on one post. Laravel allowed ten a minute.
var ErrRateLimited = errors.New("postaccess: too many attempts")

// RateLimit and RateWindow match GameController::access's RateLimiter.
const (
	RateLimit  = 10
	RateWindow = time.Minute
)

// Post is what deciding access needs to know.
type Post struct {
	ID             int64
	Serial         string
	OwnerID        int64
	Policy         string
	PasswordDigest string
}

// RequiresPassword reports whether a visitor must prove a door code.
func (post Post) RequiresPassword() bool { return post.Policy == PolicyPassword }

// IsPublic reports whether anyone may see it.
func (post Post) IsPublic() bool { return post.Policy == PolicyPublic }

// Store reads the policy behind a serial.
type Store interface {
	Post(ctx context.Context, serial string) (Post, error)
}

// Attempts limits how often one post's password may be guessed.
type Attempts interface {
	// Allow records an attempt and reports whether it was within the budget.
	Allow(ctx context.Context, serial string) (bool, error)
}

// Service grants and checks post access.
type Service struct {
	store    Store
	signer   *Signer
	attempts Attempts
}

type ServiceOptions struct {
	Store  Store
	Signer *Signer
	// Attempts is optional. Without it the password is not rate limited, which is the
	// state of a deployment with no Redis — worth knowing, because a door code is short.
	Attempts Attempts
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Store == nil {
		return nil, errors.New("postaccess: store is required")
	}
	if options.Signer == nil {
		return nil, errors.New("postaccess: signer is required")
	}
	return &Service{store: options.Store, signer: options.Signer, attempts: options.Attempts}, nil
}

// Grant checks a password and issues a token proving it was known.
//
// The rate limit is charged before the password is compared, so guessing costs the same
// whether or not the guess was close — and a post that is not password-protected does not
// spend anyone's budget, because it is refused before that point.
func (service *Service) Grant(
	ctx context.Context, serial, password string,
) (token string, expiresAt time.Time, err error) {
	post, err := service.store.Post(ctx, serial)
	if err != nil {
		return "", time.Time{}, err
	}
	if !post.RequiresPassword() {
		// Nothing to unlock. Answered as a wrong password rather than "this post has no
		// password", which would tell a stranger which posts are protected.
		return "", time.Time{}, ErrWrongPassword
	}

	if service.attempts != nil {
		allowed, err := service.attempts.Allow(ctx, serial)
		if err != nil {
			return "", time.Time{}, err
		}
		if !allowed {
			return "", time.Time{}, ErrRateLimited
		}
	}

	if !PasswordMatches(password, post.PasswordDigest) {
		return "", time.Time{}, ErrWrongPassword
	}

	token, expiresAt = service.signer.Issue(post.Serial)
	return token, expiresAt, nil
}

// Caller is what one request has proved about itself.
//
// It is passed down into the queries rather than checked before them: the policy join
// then decides visibility in the same statement that reads the row, so there is no window
// between the check and the read, and no query can forget to apply it.
type Caller struct {
	// UserID is the signed-in account, or 0 for a visitor.
	UserID int64
	// UnlockedSerials are the posts whose password this request proved.
	UnlockedSerials []string
}

// Unlocked reports whether the caller proved serial's password.
func (caller Caller) Unlocked(serial string) bool {
	for _, unlocked := range caller.UnlockedSerials {
		if unlocked == serial {
			return true
		}
	}
	return false
}

// CallerFor builds the Caller for one request from the tokens it presented.
//
// A token that does not verify is dropped silently: the caller simply has not proved that
// post, and the query refuses them the same way it refuses anyone else. Answering "your
// token is bad" would be a hint to someone probing.
func (service *Service) CallerFor(userID int64, serialsToTokens map[string]string) Caller {
	caller := Caller{UserID: userID}
	for serial, token := range serialsToTokens {
		if service.signer.Verify(serial, token) == nil {
			caller.UnlockedSerials = append(caller.UnlockedSerials, serial)
		}
	}
	return caller
}

// Reissue mints a fresh token for a serial the caller has already proved.
//
// This is AccessTokenService::extendPostAccessToken: Laravel pushed the session entry's
// expiry forward on every use, so a visitor part-way through a long game was not thrown
// out. Statelessly the same thing is a new token handed back on the response.
func (service *Service) Reissue(serial string) (string, time.Time) {
	return service.signer.Issue(serial)
}

// Verify reports whether a token proves serial.
func (service *Service) Verify(serial, token string) error {
	return service.signer.Verify(serial, token)
}

// VisibilityClause is the SQL predicate deciding whether a caller may see a post.
//
// Written once and used by every query that reads a post, so "public, or mine, or one I
// unlocked" cannot drift between them. The arguments it needs are returned alongside.
//
// postAlias and policyAlias are the table aliases in the caller's statement.
func VisibilityClause(postAlias, policyAlias string, caller Caller) (clause string, arguments []any) {
	// Public is first because it is the overwhelming majority: 2,588 posts and 307,294 of
	// the last quarter's 309,000 games.
	clause = fmt.Sprintf("(%s.access_policy = 'public'", policyAlias)
	if caller.UserID != 0 {
		clause += fmt.Sprintf(" OR %s.user_id = ?", postAlias)
		arguments = append(arguments, caller.UserID)
	}
	if len(caller.UnlockedSerials) > 0 {
		placeholders := ""
		for index, serial := range caller.UnlockedSerials {
			if index > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			arguments = append(arguments, serial)
		}
		clause += fmt.Sprintf(" OR (%s.access_policy = '%s' AND %s.serial IN (%s))",
			policyAlias, PolicyPassword, postAlias, placeholders)
	}
	return clause + ")", arguments
}
