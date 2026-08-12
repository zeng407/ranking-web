package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MySQLAccountStore implements AccountStore.
type MySQLAccountStore struct {
	database *sql.DB
}

func NewMySQLAccountStore(database *sql.DB) *MySQLAccountStore {
	return &MySQLAccountStore{database: database}
}

// accountQuery reads the settings view in one round trip.
//
// The Google link is an EXISTS rather than a join: user_socialities has one row per
// provider and a join would multiply the account row once a second provider exists.
const accountQuery = `
	SELECT u.name,
	       u.email,
	       COALESCE(u.avatar_url, ''),
	       u.password <> '',
	       EXISTS (SELECT 1 FROM user_socialities AS s WHERE s.user_id = u.id),
	       u.name_updated_at
	  FROM users AS u
	 WHERE u.id = ?
	 LIMIT 1`

func (store *MySQLAccountStore) Account(ctx context.Context, userID int64) (Account, error) {
	var (
		account       Account
		nameChangedAt sql.NullTime
	)
	err := store.database.QueryRowContext(ctx, accountQuery, userID).Scan(
		&account.Name, &account.Email, &account.AvatarURL,
		&account.HasPassword, &account.GoogleLinked, &nameChangedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrUserNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("auth: read account %d: %w", userID, err)
	}
	if nameChangedAt.Valid {
		account.NameChangedAt = nameChangedAt.Time
	}
	return account, nil
}

// updateNameStatement carries the rate limit in its WHERE clause.
//
// THE AFFECTED-ROW COUNT IS ONLY TRUSTWORTHY BECAUSE THE NAME IS KNOWN TO DIFFER. MySQL
// counts rows *changed*, not rows *matched*, so a statement that writes the values
// already present reports zero and would read as "the limit refused it". The caller
// returns early when the submitted name equals the stored one, which leaves name as a
// column that always changes here.
const updateNameStatement = `
	UPDATE users
	   SET name = ?,
	       name_updated_at = ?,
	       updated_at = ?
	 WHERE id = ?
	   AND (name_updated_at IS NULL OR name_updated_at < ?)`

func (store *MySQLAccountStore) UpdateName(
	ctx context.Context, userID int64, name string, notChangedSince, changedAt time.Time,
) (bool, error) {
	result, err := store.database.ExecContext(ctx, updateNameStatement,
		name, changedAt, changedAt, userID, notChangedSince)
	if err != nil {
		return false, fmt.Errorf("auth: update name for %d: %w", userID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("auth: update name for %d: %w", userID, err)
	}
	return affected > 0, nil
}

const updateAvatarStatement = `UPDATE users SET avatar_url = ?, updated_at = ? WHERE id = ?`

func (store *MySQLAccountStore) UpdateAvatarURL(ctx context.Context, userID int64, url string) error {
	if _, err := store.database.ExecContext(ctx, updateAvatarStatement,
		url, time.Now(), userID); err != nil {
		return fmt.Errorf("auth: update avatar for %d: %w", userID, err)
	}
	return nil
}

const updatePasswordStatement = `UPDATE users SET password = ?, updated_at = ? WHERE id = ?`

// UpdatePasswordHash writes the hash.
//
// No affected-row check: the hash is freshly generated with a random salt, so it cannot
// match what is stored, but a caller who somehow wrote the same value twice would still
// have got what they asked for. The row is known to exist — FindByID read it moments ago.
func (store *MySQLAccountStore) UpdatePasswordHash(ctx context.Context, userID int64, hash string) error {
	if _, err := store.database.ExecContext(ctx, updatePasswordStatement,
		hash, time.Now(), userID); err != nil {
		return fmt.Errorf("auth: update password for %d: %w", userID, err)
	}
	return nil
}
