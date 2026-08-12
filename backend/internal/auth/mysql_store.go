package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"2pick.app/backend/internal/platform/mysqlstore"
)

// ErrUserNotFound means no account matches. Callers must not surface it: telling a
// caller that an address is unregistered turns the login form into an account
// enumeration oracle.
var ErrUserNotFound = errors.New("auth: user not found")

// Credentials is what the login path needs about an account.
type Credentials struct {
	UserID int64
	// PasswordHash is the bcrypt hash as Laravel stored it, or "" for an account
	// that has never had a password. 11,040 of the 13,396 accounts in production are
	// in that second state: they signed in through Google or Twitch and the column
	// is an empty string, not NULL.
	PasswordHash string
	Roles        []string
}

// UserStore reads accounts.
type UserStore interface {
	FindByEmail(ctx context.Context, email string) (Credentials, error)
	FindByID(ctx context.Context, userID int64) (Credentials, error)
}

// MySQLUserStore implements UserStore.
type MySQLUserStore struct {
	database *sql.DB
}

func NewMySQLUserStore(database *sql.DB) *MySQLUserStore {
	return &MySQLUserStore{database: database}
}

// Laravel compares e-mail addresses with the column's collation, which is
// utf8mb4_unicode_ci — case-insensitive. Lower-casing in Go instead would be a
// different rule and could let two accounts collide, so the comparison is left to
// MySQL.
const credentialsByEmailQuery = `SELECT id, password FROM users WHERE email = ? LIMIT 1`

const credentialsByIDQuery = `SELECT id, password FROM users WHERE id = ? LIMIT 1`

const rolesForUserQuery = `
	SELECT r.slug
	  FROM user_roles AS ur
	  JOIN roles AS r ON r.id = ur.role_id
	 WHERE ur.user_id = ?
	 ORDER BY r.id`

func (store *MySQLUserStore) FindByEmail(ctx context.Context, email string) (Credentials, error) {
	return store.find(ctx, credentialsByEmailQuery, email)
}

func (store *MySQLUserStore) FindByID(ctx context.Context, userID int64) (Credentials, error) {
	return store.find(ctx, credentialsByIDQuery, userID)
}

func (store *MySQLUserStore) find(ctx context.Context, query string, argument any) (Credentials, error) {
	var credentials Credentials
	err := store.database.QueryRowContext(ctx, query, argument).
		Scan(&credentials.UserID, &credentials.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Credentials{}, ErrUserNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: look up account: %w", err)
	}

	roles, err := rolesForUser(ctx, store.database, credentials.UserID)
	if err != nil {
		return Credentials{}, err
	}
	credentials.Roles = roles
	return credentials, nil
}

// rolesForUser is shared with MySQLSocialStore: an account reached through OAuth gets
// the same roles as one reached through a password, and having two copies of this
// query is how the two paths would drift.
func rolesForUser(ctx context.Context, database *sql.DB, userID int64) ([]string, error) {
	rows, err := database.QueryContext(ctx, rolesForUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: read roles for user %d: %w", userID, err)
	}
	defer rows.Close()

	// Empty rather than nil: the claim must encode as [] to match what
	// GoAccessTokenService produces.
	roles := make([]string, 0, 2)
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("auth: scan role: %w", err)
		}
		roles = append(roles, slug)
	}
	return roles, rows.Err()
}

// emailExistsQuery leaves the comparison to the column's collation, which is
// case-insensitive, for the same reason FindByEmail does.
const emailExistsQuery = `SELECT 1 FROM users WHERE email = ? LIMIT 1`

// emailExists is shared by the registration path and the OAuth path. One query, so the
// two cannot disagree about what "already taken" means.
func emailExists(ctx context.Context, database *sql.DB, email string) (bool, error) {
	var found int
	err := database.QueryRowContext(ctx, emailExistsQuery, strings.TrimSpace(email)).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("auth: check whether an address is taken: %w", err)
	}
	return true, nil
}

func (store *MySQLUserStore) EmailExists(ctx context.Context, email string) (bool, error) {
	return emailExists(ctx, store.database, email)
}

// insertPasswordUserStatement creates an account that has a password.
//
// avatar_url is left NULL: a password sign-up has no picture to copy, unlike the OAuth
// path. email_verified_at is left NULL too, matching Laravel — the User model does not
// implement MustVerifyEmail, so nothing ever sets it for these accounts.
const insertPasswordUserStatement = `
	INSERT INTO users (name, email, password, created_at, updated_at)
	VALUES (?, ?, ?, NOW(), NOW())`

func (store *MySQLUserStore) CreateUser(ctx context.Context, record NewUser) (Credentials, error) {
	result, err := store.database.ExecContext(ctx, insertPasswordUserStatement,
		record.Name, record.Email, record.PasswordHash)
	if err != nil {
		if mysqlstore.IsDuplicateKey(err) {
			// The unique index on the address caught a race with another sign-up.
			return Credentials{}, ErrOAuthEmailTaken
		}
		return Credentials{}, fmt.Errorf("auth: create account: %w", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: new account id: %w", err)
	}
	// Empty rather than nil so the roles claim encodes as [] and not null.
	return Credentials{UserID: userID, PasswordHash: record.PasswordHash, Roles: []string{}}, nil
}

// MySQLRefreshStore implements RefreshStore.
type MySQLRefreshStore struct {
	database *sql.DB
}

func NewMySQLRefreshStore(database *sql.DB) *MySQLRefreshStore {
	return &MySQLRefreshStore{database: database}
}

const insertRefreshTokenStatement = `
	INSERT INTO go_refresh_tokens
	       (user_id, family_id, token_hash, csrf_hash, issued_at, expires_at,
	        created_ip, user_agent, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

func (store *MySQLRefreshStore) Create(ctx context.Context, record NewSession) (Session, error) {
	result, err := store.database.ExecContext(ctx, insertRefreshTokenStatement,
		record.UserID, record.FamilyID, record.TokenHash, record.CSRFHash,
		record.IssuedAt.UTC(), record.ExpiresAt.UTC(),
		nullableString(record.CreatedIP), nullableString(truncate(record.UserAgent, 255)))
	if err != nil {
		return Session{}, fmt.Errorf("auth: store refresh token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("auth: refresh token id: %w", err)
	}
	return Session{
		ID:        id,
		UserID:    record.UserID,
		FamilyID:  record.FamilyID,
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
		CSRFHash:  record.CSRFHash,
	}, nil
}

const findRefreshTokenQuery = `
	SELECT id, user_id, family_id, csrf_hash, issued_at, expires_at, used_at, revoked_at
	  FROM go_refresh_tokens
	 WHERE token_hash = ?
	 LIMIT 1`

func (store *MySQLRefreshStore) FindByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	var (
		session   Session
		usedAt    sql.NullTime
		revokedAt sql.NullTime
	)
	err := store.database.QueryRowContext(ctx, findRefreshTokenQuery, tokenHash).Scan(
		&session.ID, &session.UserID, &session.FamilyID, &session.CSRFHash,
		&session.IssuedAt, &session.ExpiresAt, &usedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrRefreshTokenInvalid
	}
	if err != nil {
		return Session{}, fmt.Errorf("auth: look up refresh token: %w", err)
	}
	if usedAt.Valid {
		session.UsedAt = &usedAt.Time
	}
	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}
	return session, nil
}

// markUsedStatement only consumes a token that has not been consumed already. The
// WHERE clause is what makes rotation safe under concurrency: two simultaneous
// refreshes with the same token both read an unused row, but only one UPDATE
// matches, and the loser sees zero rows affected and is treated as a replay.
const markUsedStatement = `
	UPDATE go_refresh_tokens
	   SET used_at = ?, updated_at = NOW()
	 WHERE id = ? AND used_at IS NULL`

func (store *MySQLRefreshStore) MarkUsed(ctx context.Context, sessionID int64, usedAt time.Time) error {
	result, err := store.database.ExecContext(ctx, markUsedStatement, usedAt.UTC(), sessionID)
	if err != nil {
		return fmt.Errorf("auth: consume refresh token: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		// Someone else consumed it between the read and this write.
		return ErrRefreshTokenReused
	}
	return nil
}

const revokeFamilyStatement = `
	UPDATE go_refresh_tokens
	   SET revoked_at = ?, updated_at = NOW()
	 WHERE family_id = ? AND revoked_at IS NULL`

func (store *MySQLRefreshStore) RevokeFamily(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	result, err := store.database.ExecContext(ctx, revokeFamilyStatement, revokedAt.UTC(), familyID)
	if err != nil {
		return 0, fmt.Errorf("auth: revoke session family: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

const revokeUserStatement = `
	UPDATE go_refresh_tokens
	   SET revoked_at = ?, updated_at = NOW()
	 WHERE user_id = ? AND revoked_at IS NULL`

func (store *MySQLRefreshStore) RevokeUser(ctx context.Context, userID int64, revokedAt time.Time) (int64, error) {
	result, err := store.database.ExecContext(ctx, revokeUserStatement, revokedAt.UTC(), userID)
	if err != nil {
		return 0, fmt.Errorf("auth: revoke user sessions: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// deleteExpiredStatement is bounded so a cleanup cannot lock the table while it
// deletes months of rows in one statement.
const deleteExpiredStatement = `DELETE FROM go_refresh_tokens WHERE expires_at < ? LIMIT ?`

func (store *MySQLRefreshStore) DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	result, err := store.database.ExecContext(ctx, deleteExpiredStatement, before.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("auth: purge expired refresh tokens: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
