package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MySQLPasswordResetStore implements PasswordResetStore against go_password_resets.
type MySQLPasswordResetStore struct {
	database *sql.DB
}

func NewMySQLPasswordResetStore(database *sql.DB) *MySQLPasswordResetStore {
	return &MySQLPasswordResetStore{database: database}
}

const insertPasswordResetStatement = `
	INSERT INTO go_password_resets
	       (user_id, token_hash, requested_at, expires_at, requested_ip)
	VALUES (?, ?, ?, ?, ?)`

func (store *MySQLPasswordResetStore) Create(ctx context.Context, record NewPasswordReset) error {
	if _, err := store.database.ExecContext(ctx, insertPasswordResetStatement,
		record.UserID, record.TokenHash, record.RequestedAt.UTC(), record.ExpiresAt.UTC(),
		nullableString(truncate(record.RequestedIP, 45))); err != nil {
		return fmt.Errorf("auth: store password reset for %d: %w", record.UserID, err)
	}
	return nil
}

const lastPasswordResetQuery = `
	SELECT requested_at
	  FROM go_password_resets
	 WHERE user_id = ?
	 ORDER BY requested_at DESC
	 LIMIT 1`

func (store *MySQLPasswordResetStore) LastRequestedAt(
	ctx context.Context, userID int64,
) (time.Time, bool, error) {
	var requestedAt time.Time
	err := store.database.QueryRowContext(ctx, lastPasswordResetQuery, userID).Scan(&requestedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("auth: read last password reset for %d: %w", userID, err)
	}
	return requestedAt, true, nil
}

// claimPasswordResetStatement is the whole single-use guard.
//
// Same shape as markUsedStatement above and for the same reason: two requests carrying
// the same token both read an unused row, but only one UPDATE matches it, so only one of
// them gets to set a password. Expiry is in the WHERE clause too, so a stale link fails
// the claim rather than needing a separate check the caller could forget.
const claimPasswordResetStatement = `
	UPDATE go_password_resets
	   SET used_at = ?
	 WHERE token_hash = ?
	   AND used_at IS NULL
	   AND expires_at > ?`

// userIDByPasswordResetQuery reads the owner of a token the caller has already claimed.
//
// A second statement because MySQL's UPDATE cannot return a column. It is not a race:
// the row is claimed by the time this runs, so nothing else can be acting on it, and
// token_hash is unique so it names exactly one row.
const userIDByPasswordResetQuery = `
	SELECT user_id FROM go_password_resets WHERE token_hash = ? LIMIT 1`

func (store *MySQLPasswordResetStore) Consume(
	ctx context.Context, tokenHash string, at time.Time,
) (int64, error) {
	result, err := store.database.ExecContext(ctx, claimPasswordResetStatement,
		at.UTC(), tokenHash, at.UTC())
	if err != nil {
		return 0, fmt.Errorf("auth: claim password reset: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("auth: claim password reset: %w", err)
	}
	if affected == 0 {
		// Unknown, expired, or claimed by someone else. The caller reports all three the
		// same way; see ErrResetTokenInvalid.
		return 0, ErrResetTokenInvalid
	}

	var userID int64
	if err := store.database.QueryRowContext(ctx, userIDByPasswordResetQuery, tokenHash).
		Scan(&userID); err != nil {
		return 0, fmt.Errorf("auth: read password reset owner: %w", err)
	}
	return userID, nil
}
