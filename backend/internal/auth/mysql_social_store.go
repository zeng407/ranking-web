package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"2pick.app/backend/internal/platform/mysqlstore"
)

// ProviderGoogle is the only provider this application has ever supported.
//
// The Laravel route /auth/twitch/callback exists but its handler returns the literal
// string "Twitch callback", and user_socialities has no twitch columns at all — so
// there is nothing to port. TWITCH_CLIENT_ID in the environment belongs to the
// refresh:token artisan command, which reads the streams API; it is not a login.
const ProviderGoogle = "google"

// MySQLSocialStore implements SocialStore against the existing user_socialities table.
//
// The table is column-per-provider (google_id, google_email, ...) rather than a
// (provider, subject) pair, which is why every method starts by rejecting an unknown
// provider instead of parameterising the column name. Adding a provider here is a
// schema change, and building the column name from a string would be an injection
// vector for no benefit.
type MySQLSocialStore struct {
	database *sql.DB
}

func NewMySQLSocialStore(database *sql.DB) *MySQLSocialStore {
	return &MySQLSocialStore{database: database}
}

// ErrUnknownProvider is a programming error, not a user-facing one.
var ErrUnknownProvider = errors.New("auth: unknown identity provider")

func requireGoogle(provider string) error {
	if provider != ProviderGoogle {
		return fmt.Errorf("%w: %q", ErrUnknownProvider, provider)
	}
	return nil
}

const credentialsByGoogleIDQuery = `
	SELECT u.id, u.password
	  FROM user_socialities AS s
	  JOIN users AS u ON u.id = s.user_id
	 WHERE s.google_id = ?
	 LIMIT 1`

func (store *MySQLSocialStore) FindByProviderSubject(
	ctx context.Context, provider, subject string,
) (Credentials, error) {
	if err := requireGoogle(provider); err != nil {
		return Credentials{}, err
	}
	if subject == "" {
		// An empty subject would match the rows where google_id is NULL under some
		// comparisons, and means nothing in any case.
		return Credentials{}, ErrUserNotFound
	}

	var credentials Credentials
	err := store.database.QueryRowContext(ctx, credentialsByGoogleIDQuery, subject).
		Scan(&credentials.UserID, &credentials.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Credentials{}, ErrUserNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: look up %s subject: %w", provider, err)
	}

	roles, err := rolesForUser(ctx, store.database, credentials.UserID)
	if err != nil {
		return Credentials{}, err
	}
	credentials.Roles = roles
	return credentials, nil
}

// EmailExists shares its query with the registration path; see emailExists.
func (store *MySQLSocialStore) EmailExists(ctx context.Context, email string) (bool, error) {
	return emailExists(ctx, store.database, email)
}

// insertUserStatement writes password = ” for the same reason Laravel does: the
// column is NOT NULL and these accounts have no password. The login path refuses to
// compare against an empty hash.
//
// email_verified_at carries the provider's claim, matching what SocialiteService
// wrote. remember_token is left NULL: it belongs to Laravel's cookie session, which
// this path does not create.
const insertUserStatement = `
	INSERT INTO users (name, email, password, avatar_url, email_verified_at, created_at, updated_at)
	VALUES (?, ?, '', ?, ?, ?, ?)`

const insertSocialiteStatement = `
	INSERT INTO user_socialities (user_id, google_email, google_id, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)`

func (store *MySQLSocialStore) CreateLinkedUser(
	ctx context.Context, record NewLinkedUser,
) (Credentials, error) {
	if err := requireGoogle(record.Provider); err != nil {
		return Credentials{}, err
	}

	// One transaction for both rows. An account with no link would be unreachable:
	// it has no password either, so nothing could ever sign into it, and its address
	// would block the retry that would otherwise fix it.
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: begin account creation: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	var verifiedAt any
	if record.EmailVerified {
		verifiedAt = record.CreatedAt
	}

	result, err := transaction.ExecContext(ctx, insertUserStatement,
		record.Name, record.Email, nullableString(record.AvatarURL), verifiedAt,
		record.CreatedAt, record.CreatedAt)
	if err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			// The address was taken between the EmailExists check and here, or two
			// callbacks for the same new account arrived together. Either way the
			// answer the caller wants is the same one the check would have given.
			return Credentials{}, ErrOAuthEmailTaken
		}
		return Credentials{}, fmt.Errorf("auth: create account: %w", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: new account id: %w", err)
	}

	if _, err := transaction.ExecContext(ctx, insertSocialiteStatement,
		userID, record.Email, record.Subject, record.CreatedAt, record.CreatedAt); err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			// The unique index added in 00009 firing: a concurrent callback for the
			// same Google account got there first. Rolling back and reporting
			// "already linked" lets the caller retry the lookup and find that row.
			return Credentials{}, ErrOAuthAlreadyLinked
		}
		return Credentials{}, fmt.Errorf("auth: link %s account: %w", record.Provider, err)
	}

	if err := transaction.Commit(); err != nil {
		return Credentials{}, fmt.Errorf("auth: commit account creation: %w", err)
	}

	// A brand new account has no roles. Empty rather than nil so the claim encodes
	// as [] and not null.
	return Credentials{UserID: userID, PasswordHash: "", Roles: []string{}}, nil
}

const socialiteRowForUserQuery = `
	SELECT id, google_id IS NOT NULL
	  FROM user_socialities
	 WHERE user_id = ?
	 LIMIT 1`

const attachGoogleToRowStatement = `
	UPDATE user_socialities
	   SET google_id = ?, google_email = ?, updated_at = NOW()
	 WHERE id = ? AND google_id IS NULL`

func (store *MySQLSocialStore) Link(ctx context.Context, request LinkRequest) error {
	if err := requireGoogle(request.Provider); err != nil {
		return err
	}
	if request.UserID <= 0 || request.Subject == "" {
		return ErrOAuthAlreadyLinked
	}

	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin link: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	var (
		rowID       int64
		alreadyHeld bool
	)
	err = transaction.QueryRowContext(ctx, socialiteRowForUserQuery, request.UserID).
		Scan(&rowID, &alreadyHeld)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No socialite row yet: create one. The unique index is what arbitrates if
		// another user is linking the same Google account at the same moment.
		now := time.Now().UTC()
		if _, err := transaction.ExecContext(ctx, insertSocialiteStatement,
			request.UserID, request.Email, request.Subject, now, now); err != nil {
			if mysqlstore.IsDuplicateKey(err) {
				return ErrOAuthAlreadyLinked
			}
			return fmt.Errorf("auth: link %s account: %w", request.Provider, err)
		}
	case err != nil:
		return fmt.Errorf("auth: read the existing link: %w", err)
	case alreadyHeld:
		// This user already has a Google account attached. Replacing it silently
		// would move the login of whoever owns the old one.
		return ErrOAuthAlreadyLinked
	default:
		result, err := transaction.ExecContext(ctx, attachGoogleToRowStatement,
			request.Subject, request.Email, rowID)
		if err != nil {
			if mysqlstore.IsDuplicateKey(err) {
				return ErrOAuthAlreadyLinked
			}
			return fmt.Errorf("auth: attach %s account: %w", request.Provider, err)
		}
		// The WHERE google_id IS NULL clause makes this safe under concurrency the
		// same way MarkUsed is. Zero rows means another request attached one first.
		//
		// Rows *changed*, not matched — but google_id going from NULL to a value is
		// always a change, so the distinction that makes MarkUsed subtle does not
		// arise here.
		if affected, _ := result.RowsAffected(); affected == 0 {
			return ErrOAuthAlreadyLinked
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("auth: commit link: %w", err)
	}
	return nil
}
