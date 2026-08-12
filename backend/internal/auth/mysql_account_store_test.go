package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The account store against the real server. The rate limit lives in a WHERE clause and
// is read back through an affected-row count, which is the same MySQL behaviour that
// made two of the refresh-token tests vacuous: the count is rows *changed*, not rows
// *matched*. These tests exist to keep that from being re-learned the hard way.

type accountFixture struct {
	database  *sql.DB
	store     *MySQLAccountStore
	namespace string
}

func newAccountFixture(t *testing.T) (*accountFixture, context.Context) {
	t.Helper()
	database := testDatabase(t)

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("namespace: %v", err)
	}
	fixture := &accountFixture{
		database:  database,
		store:     NewMySQLAccountStore(database),
		namespace: "go-account-" + hex.EncodeToString(suffix),
	}
	t.Cleanup(func() {
		// The socialities first: the rows point at the users about to go.
		if _, err := database.Exec(
			`DELETE s FROM user_socialities AS s JOIN users AS u ON u.id = s.user_id
			  WHERE u.email LIKE ?`, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up test socialities: %v", err)
		}
		if _, err := database.Exec(
			`DELETE FROM users WHERE email LIKE ?`, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up test users: %v", err)
		}
	})
	return fixture, context.Background()
}

// createUser makes a throwaway account. passwordHash of "" is the shape 11,040
// production accounts are in.
func (fixture *accountFixture) createUser(
	t *testing.T, ctx context.Context, name, passwordHash string, nameChangedAt *time.Time,
) int64 {
	t.Helper()
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO users (name, email, password, name_updated_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, NOW(), NOW())`,
		name, fmt.Sprintf("%s-%s@invalid.test", fixture.namespace, name), passwordHash, nameChangedAt)
	if err != nil {
		t.Fatalf("create a test user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("test user id: %v", err)
	}
	return userID
}

func (fixture *accountFixture) storedName(t *testing.T, ctx context.Context, userID int64) string {
	t.Helper()
	var name string
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT name FROM users WHERE id = ?`, userID).Scan(&name); err != nil {
		t.Fatalf("read back the name: %v", err)
	}
	return name
}

func TestAccountReadsEveryFieldTheSettingsPageDraws(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	changedAt := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	userID := fixture.createUser(t, ctx, "reader", "$2y$10$notarealhashbutthecolumnfits", &changedAt)

	account, err := fixture.store.Account(ctx, userID)
	if err != nil {
		t.Fatalf("Account() error = %v", err)
	}
	if account.Name != "reader" {
		t.Errorf("Name = %q, want %q", account.Name, "reader")
	}
	if !strings.HasPrefix(account.Email, fixture.namespace) {
		t.Errorf("Email = %q, want the fixture address", account.Email)
	}
	if !account.HasPassword {
		t.Error("HasPassword = false for an account with a hash")
	}
	if account.GoogleLinked {
		t.Error("GoogleLinked = true for an account with no socialities row")
	}
	// Second precision: the column is a TIMESTAMP and the driver runs in Asia/Taipei
	// against UTC columns, so a mismatch here is a timezone bug, not a rounding one.
	if delta := account.NameChangedAt.Sub(changedAt); delta > time.Second || delta < -time.Second {
		t.Errorf("NameChangedAt = %v, want %v", account.NameChangedAt, changedAt)
	}
}

// The empty-password state is the majority of production, and the settings page decides
// which of the two password endpoints to offer from it.
func TestAccountReportsAnEmptyPasswordAsNoPassword(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	userID := fixture.createUser(t, ctx, "google-only", "", nil)

	account, err := fixture.store.Account(ctx, userID)
	if err != nil {
		t.Fatalf("Account() error = %v", err)
	}
	if account.HasPassword {
		t.Error("HasPassword = true for an account whose password column is empty")
	}
	if !account.NameChangedAt.IsZero() {
		t.Errorf("NameChangedAt = %v, want the zero time for a NULL column", account.NameChangedAt)
	}
}

func TestAccountReportsAGoogleLink(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	userID := fixture.createUser(t, ctx, "linked", "", nil)
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO user_socialities (user_id, google_id, google_token, created_at, updated_at)
		 VALUES (?, ?, '', NOW(), NOW())`, userID, fixture.namespace+"-sub"); err != nil {
		t.Fatalf("link a provider: %v", err)
	}

	account, err := fixture.store.Account(ctx, userID)
	if err != nil {
		t.Fatalf("Account() error = %v", err)
	}
	if !account.GoogleLinked {
		t.Error("GoogleLinked = false with a socialities row present")
	}
}

func TestAccountOfAMissingUserIsErrUserNotFound(t *testing.T) {
	fixture, ctx := newAccountFixture(t)

	if _, err := fixture.store.Account(ctx, -1); err != ErrUserNotFound {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

// The rate limit is the WHERE clause, so this is the test that proves it is enforced by
// the server rather than by the caller's reading of the row.
func TestUpdateNameRefusesASecondChangeInsideTheWindow(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	userID := fixture.createUser(t, ctx, "first", "", nil)

	now := time.Now()
	boundary := now.Add(-24 * time.Hour)

	written, err := fixture.store.UpdateName(ctx, userID, "second", boundary, now)
	if err != nil {
		t.Fatalf("first UpdateName() error = %v", err)
	}
	if !written {
		t.Fatal("the first change was refused; a NULL name_updated_at must pass the guard")
	}
	if name := fixture.storedName(t, ctx, userID); name != "second" {
		t.Fatalf("stored name = %q, want %q", name, "second")
	}

	written, err = fixture.store.UpdateName(ctx, userID, "third", boundary, now)
	if err != nil {
		t.Fatalf("second UpdateName() error = %v", err)
	}
	if written {
		t.Error("the second change inside the window was allowed")
	}
	if name := fixture.storedName(t, ctx, userID); name != "second" {
		t.Errorf("stored name = %q; the refused change must not have been written", name)
	}
}

func TestUpdateNameAllowsAChangeOnceTheWindowHasPassed(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	yesterday := time.Now().Add(-30 * time.Hour)
	userID := fixture.createUser(t, ctx, "old", "", &yesterday)

	written, err := fixture.store.UpdateName(ctx, userID, "new",
		time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("UpdateName() error = %v", err)
	}
	if !written {
		t.Fatal("a change was refused although the last one was 30 hours ago")
	}
}

// THE AFFECTED-ROW TRAP, DEMONSTRATED. MySQL counts rows *changed*, not rows *matched*,
// so a statement whose values are all already present reports zero — and this store
// reads zero as "the rate limit refused it".
//
// Every column the statement writes has to match for that to happen, updated_at
// included, which is why it takes a row rigged like this to produce it. That is exactly
// why the service returns early when the submitted name equals the stored one: with the
// name known to differ, at least one column always changes and the count means what the
// store claims it means. Without that guard, a user resubmitting the settings form
// unchanged would be told they had hit a limit they had not hit.
func TestUpdateNameWithEveryColumnAlreadyEqualReportsNotWritten(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	// Second precision: the columns are TIMESTAMPs and MySQL rounds sub-seconds rather
	// than truncating them, so a stamp with a fraction would not compare equal.
	stamp := time.Now().Truncate(time.Second)
	userID := fixture.createUser(t, ctx, "same", "", &stamp)
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE users SET updated_at = ? WHERE id = ?`, stamp, userID); err != nil {
		t.Fatalf("rig updated_at: %v", err)
	}

	// Same name, same stamp, same updated_at: nothing in the row changes.
	written, err := fixture.store.UpdateName(ctx, userID, "same", stamp.Add(time.Second), stamp)
	if err != nil {
		t.Fatalf("UpdateName() error = %v", err)
	}
	if written {
		t.Fatal("MySQL reported a row changed when every written column already held " +
			"its value; the store's reading of the affected-row count assumes otherwise")
	}
	if name := fixture.storedName(t, ctx, userID); name != "same" {
		t.Errorf("stored name = %q, want %q", name, "same")
	}
}

// Two requests in the same instant. Exactly one may win: the guard is in the statement,
// so the second one's UPDATE matches no row.
func TestConcurrentNameChangesLetOneThrough(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	userID := fixture.createUser(t, ctx, "contended", "", nil)

	const attempts = 8
	var (
		results = make(chan bool, attempts)
		now     = time.Now()
		start   = make(chan struct{})
	)
	for attempt := 0; attempt < attempts; attempt++ {
		go func(attempt int) {
			<-start
			written, err := fixture.store.UpdateName(ctx, userID,
				fmt.Sprintf("racer-%d", attempt), now.Add(-24*time.Hour), now)
			if err != nil {
				t.Errorf("UpdateName() error = %v", err)
			}
			results <- written
		}(attempt)
	}
	close(start)

	won := 0
	for attempt := 0; attempt < attempts; attempt++ {
		if <-results {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("%d of %d concurrent changes were written, want exactly 1", won, attempts)
	}
}

func TestUpdateAvatarURLAndPasswordHashWriteWhatTheyAreGiven(t *testing.T) {
	fixture, ctx := newAccountFixture(t)
	userID := fixture.createUser(t, ctx, "writer", "", nil)

	const url = "https://file.2pick.app/avatars/deadbeef.png"
	if err := fixture.store.UpdateAvatarURL(ctx, userID, url); err != nil {
		t.Fatalf("UpdateAvatarURL() error = %v", err)
	}
	const hash = "$2y$10$abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLM"
	if err := fixture.store.UpdatePasswordHash(ctx, userID, hash); err != nil {
		t.Fatalf("UpdatePasswordHash() error = %v", err)
	}

	account, err := fixture.store.Account(ctx, userID)
	if err != nil {
		t.Fatalf("Account() error = %v", err)
	}
	if account.AvatarURL != url {
		t.Errorf("AvatarURL = %q, want %q", account.AvatarURL, url)
	}
	if !account.HasPassword {
		t.Error("HasPassword = false after writing a hash")
	}

	// And the hash is readable by the login path, which is a different query.
	credentials, err := NewMySQLUserStore(fixture.database).FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if credentials.PasswordHash != hash {
		t.Errorf("stored hash = %q, want %q", credentials.PasswordHash, hash)
	}
}
