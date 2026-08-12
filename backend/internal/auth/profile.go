package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Profile is what a signed-in client needs to draw its own account.
//
// Deliberately small. This replaces the `user` half of Laravel's /session-context,
// which carried exactly one field the SPA read — the avatar. Name is added because a
// dropdown that shows an avatar and no name has to fall back to the address, and the
// address is the one field here worth not shipping to the browser more than necessary.
//
// No e-mail address, no roles: roles are already in the access token the caller used to
// get here, and re-serving them would create a second source for the same fact.
type Profile struct {
	UserID    int64   `json:"user_id,string"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
	// HasPassword tells the client whether to offer "change password" or "set a
	// password". 82% of accounts have none, and offering the wrong one to them is the
	// difference between a working form and a form that can never succeed.
	HasPassword bool `json:"has_password"`
	// LinkedGoogle drives the profile page's connect button.
	LinkedGoogle bool `json:"linked_google"`
}

// ProfileStore reads the account behind a token.
type ProfileStore interface {
	Profile(ctx context.Context, userID int64) (Profile, error)
}

// MySQLProfileStore implements ProfileStore.
type MySQLProfileStore struct {
	database *sql.DB
}

func NewMySQLProfileStore(database *sql.DB) *MySQLProfileStore {
	return &MySQLProfileStore{database: database}
}

// profileQuery reads the account and whether it has a Google link in one round trip.
//
// LEFT JOIN rather than a second query: an account with no socialite row is the normal
// case for the 18% who signed up with a password, and a missing row must read as "not
// linked" rather than as an error.
const profileQuery = `
	SELECT u.id, u.name, u.avatar_url, u.password <> '' AS has_password,
	       s.google_id IS NOT NULL AS linked_google
	  FROM users AS u
	  LEFT JOIN user_socialities AS s ON s.user_id = u.id
	 WHERE u.id = ?
	 LIMIT 1`

func (store *MySQLProfileStore) Profile(ctx context.Context, userID int64) (Profile, error) {
	var (
		profile   Profile
		avatarURL sql.NullString
	)
	err := store.database.QueryRowContext(ctx, profileQuery, userID).Scan(
		&profile.UserID, &profile.Name, &avatarURL, &profile.HasPassword, &profile.LinkedGoogle)
	if errors.Is(err, sql.ErrNoRows) {
		// The account was deleted while a still-valid access token was in flight. Not
		// an error worth a 500: the token names somebody who no longer exists.
		return Profile{}, ErrUserNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("auth: read profile for user %d: %w", userID, err)
	}
	if avatarURL.Valid {
		profile.AvatarURL = &avatarURL.String
	}
	return profile, nil
}
