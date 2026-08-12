package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

// These tests exist because the unit tests use a memory store that I wrote to
// reproduce the MySQL semantics. That store passing proves my model of MySQL is
// self-consistent, not that MySQL agrees with it. Everything the security argument
// leans on — that a used token cannot be consumed twice even under a real race,
// that revoking a family is idempotent, that an expiry survives the round trip
// through a driver configured in Asia/Taipei against UTC columns — is asserted here
// against the server itself.

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MYSQL_TEST_HOST is not set; skipping MySQL integration test")
	}
	port, err := strconv.Atoi(envOr("MYSQL_TEST_PORT", "3306"))
	if err != nil {
		t.Fatalf("MYSQL_TEST_PORT is not a number: %v", err)
	}

	database, err := mysqlstore.Open(config.DatabaseConfig{
		Host:            host,
		Port:            port,
		Name:            envOr("MYSQL_TEST_DATABASE", "rk_db_restore_20260729"),
		User:            envOr("MYSQL_TEST_USERNAME", "root"),
		Password:        os.Getenv("MYSQL_TEST_PASSWORD"),
		MaxOpenConns:    8,
		MaxIdleConns:    4,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("database unreachable: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// sessionFixture writes into the live table on a copy of production, so every row a
// test creates is namespaced by family id and deleted afterwards. The namespace also
// keeps concurrent test runs from revoking each other's families.
type sessionFixture struct {
	store  *MySQLRefreshStore
	userID int64
	prefix string
	nextID int
}

func newSessionFixture(t *testing.T) (*sessionFixture, context.Context) {
	t.Helper()
	database := testDatabase(t)
	ctx := context.Background()

	// A real user id: the table has a foreign key to users, so an invented one is
	// rejected. Which user does not matter — nothing here reads the account.
	var userID int64
	if err := database.QueryRowContext(ctx,
		`SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("find a user to attach sessions to: %v", err)
	}

	// Short on purpose: family_id is CHAR(36) and the namespace has to leave room for
	// the per-test suffix. Four random bytes rather than a timestamp, which would eat
	// nineteen of the thirty-six characters on its own.
	namespace := make([]byte, 4)
	if _, err := rand.Read(namespace); err != nil {
		t.Fatalf("generate a test namespace: %v", err)
	}

	fixture := &sessionFixture{
		store:  NewMySQLRefreshStore(database),
		userID: userID,
		prefix: fmt.Sprintf("gt-%x", namespace),
	}
	t.Cleanup(func() {
		if _, err := database.ExecContext(context.Background(),
			`DELETE FROM go_refresh_tokens WHERE family_id LIKE ?`, fixture.prefix+"%"); err != nil {
			t.Errorf("clean up test sessions: %v", err)
		}
	})
	return fixture, ctx
}

// family returns a namespaced family id that fits CHAR(36). MySQL in strict mode
// rejects an over-long value rather than truncating it, so the cap is applied here
// instead of being left to a test author counting characters.
func (fixture *sessionFixture) family(name string) string {
	id := fmt.Sprintf("%s-%s", fixture.prefix, name)
	if len(id) > 36 {
		id = id[:36]
	}
	return id
}

func (fixture *sessionFixture) create(t *testing.T, ctx context.Context, family string, expiresAt time.Time) (Session, string) {
	t.Helper()
	token, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	_, csrfHash, err := NewCSRFToken()
	if err != nil {
		t.Fatalf("generate csrf token: %v", err)
	}
	fixture.nextID++

	session, err := fixture.store.Create(ctx, NewSession{
		UserID:    fixture.userID,
		FamilyID:  family,
		TokenHash: hash,
		CSRFHash:  csrfHash,
		IssuedAt:  time.Now().Add(-time.Minute),
		ExpiresAt: expiresAt,
		CreatedIP: "203.0.113.7",
		UserAgent: fmt.Sprintf("go-integration-test/%d", fixture.nextID),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID == 0 {
		t.Fatal("Create returned no id; MarkUsed and RevokeFamily would target row 0")
	}
	return session, token
}

func TestRefreshStoreRoundTripsASession(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	expiresAt := time.Now().Add(RefreshTokenTTL)
	created, token := fixture.create(t, ctx, fixture.family("round-trip"), expiresAt)

	// The lookup is by hash, never by the token: the token is not in the table.
	found, err := fixture.store.FindByTokenHash(ctx, HashToken(token))
	if err != nil {
		t.Fatalf("find the session just created: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("id = %d, want %d", found.ID, created.ID)
	}
	if found.UserID != fixture.userID {
		t.Errorf("user id = %d, want %d", found.UserID, fixture.userID)
	}
	if found.FamilyID != fixture.family("round-trip") {
		t.Errorf("family = %q", found.FamilyID)
	}
	if found.CSRFHash != created.CSRFHash {
		t.Errorf("csrf hash did not survive the round trip: %q vs %q", found.CSRFHash, created.CSRFHash)
	}
	if found.Used() || found.Revoked() {
		t.Error("a freshly created session must be neither used nor revoked")
	}

	// THE TIMEZONE ROUND TRIP. The columns are TIMESTAMP, the driver is configured in
	// Asia/Taipei, and the store writes .UTC(). If those disagree the expiry comes back
	// shifted by eight hours, which either logs everyone out early or keeps dead
	// sessions alive for a third of a day.
	//
	// One second of tolerance, not zero: the column has no fractional part and MySQL
	// rounds the sub-second remainder rather than dropping it, so the stored value sits
	// on either side of the input depending on the microseconds. That is nothing to do
	// with what this assertion is looking for.
	if drift := found.ExpiresAt.Sub(expiresAt); drift > time.Second || drift < -time.Second {
		t.Errorf("expires_at came back %s off (%s vs %s); anything beyond a second of rounding is a timezone bug",
			drift, found.ExpiresAt.UTC(), expiresAt.UTC())
	}
}

// A session that has already expired must read back as inactive. This is the
// timezone assertion again, but stated as the security property rather than as a
// timestamp comparison: an eight hour shift in the wrong direction would let this
// token keep working.
func TestAnExpiredSessionReadsBackAsInactive(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	now := time.Now()

	_, expired := fixture.create(t, ctx, fixture.family("expired"), now.Add(-time.Hour))
	_, live := fixture.create(t, ctx, fixture.family("live"), now.Add(RefreshTokenTTL))

	expiredSession, err := fixture.store.FindByTokenHash(ctx, HashToken(expired))
	if err != nil {
		t.Fatalf("find the expired session: %v", err)
	}
	if expiredSession.Active(now) {
		t.Errorf("a session that expired an hour ago is still active; expires_at read back as %s, now is %s",
			expiredSession.ExpiresAt, now)
	}

	liveSession, err := fixture.store.FindByTokenHash(ctx, HashToken(live))
	if err != nil {
		t.Fatalf("find the live session: %v", err)
	}
	if !liveSession.Active(now) {
		t.Errorf("a session with thirty days left is not active; expires_at read back as %s", liveSession.ExpiresAt)
	}
}

func TestFindByTokenHashOnAnUnknownTokenIsInvalidNotAnError(t *testing.T) {
	fixture, ctx := newSessionFixture(t)

	_, err := fixture.store.FindByTokenHash(ctx, HashToken("a token that was never issued"))
	if err != ErrRefreshTokenInvalid {
		t.Fatalf("error = %v, want ErrRefreshTokenInvalid; a wrapped sql.ErrNoRows would reach the client as a 500", err)
	}
}

// MarkUsed's WHERE used_at IS NULL clause is the whole of the replay defence. Two
// presentations of the same token must not both succeed.
//
// THE TWO TIMESTAMPS MUST DIFFER. MySQL's affected-rows count is rows *changed*, not
// rows *matched*, and used_at is a TIMESTAMP with one second precision. Consume
// twice with the same second and the second UPDATE writes the value that is already
// there, so MySQL reports zero rows affected and the store answers "reused" — with
// or without the WHERE clause. Written that way this test passes against a store
// that has no replay defence at all. A second apart, only the clause can produce the
// zero.
func TestMarkUsedOnlyConsumesOnce(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	session, _ := fixture.create(t, ctx, fixture.family("consume"), time.Now().Add(RefreshTokenTTL))

	first := time.Now()
	if err := fixture.store.MarkUsed(ctx, session.ID, first); err != nil {
		t.Fatalf("first MarkUsed: %v", err)
	}
	if err := fixture.store.MarkUsed(ctx, session.ID, first.Add(10*time.Second)); err != ErrRefreshTokenReused {
		t.Fatalf("second MarkUsed error = %v, want ErrRefreshTokenReused", err)
	}
}

// The race the memory store only simulates. Eight goroutines present the same token
// at once against the real server; exactly one rotation may win.
//
// Each racer stamps a distinct second, for the reason spelled out on
// TestMarkUsedOnlyConsumesOnce: with one shared timestamp the seven losers write a
// value that is already in the column, MySQL reports nothing changed, and they look
// rejected even when nothing rejected them.
func TestConcurrentMarkUsedInMySQLAllowsExactlyOne(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	session, _ := fixture.create(t, ctx, fixture.family("race"), time.Now().Add(RefreshTokenTTL))

	const racers = 8
	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mutex   sync.Mutex
		winners int
		other   []error
	)
	base := time.Now()
	start.Add(1)
	for racer := range racers {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			err := fixture.store.MarkUsed(ctx, session.ID, base.Add(time.Duration(racer)*time.Second))
			mutex.Lock()
			defer mutex.Unlock()
			switch {
			case err == nil:
				winners++
			case err == ErrRefreshTokenReused:
			default:
				other = append(other, err)
			}
		}()
	}
	start.Done()
	done.Wait()

	if winners != 1 {
		t.Errorf("%d of %d concurrent refreshes succeeded, want exactly 1", winners, racers)
	}
	if len(other) > 0 {
		t.Errorf("unexpected errors from the losing refreshes: %v", other)
	}
}

// Revoking a family must reach every token in the chain and must not touch another
// chain. The count it returns is what the replay log reports, so it has to be the
// number actually revoked, not the number matched.
func TestRevokeFamilyRevokesTheChainOnceAndLeavesOthersAlone(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	target := fixture.family("victim")
	bystander := fixture.family("bystander")

	var tokens []string
	for range 3 {
		_, token := fixture.create(t, ctx, target, time.Now().Add(RefreshTokenTTL))
		tokens = append(tokens, token)
	}
	_, untouched := fixture.create(t, ctx, bystander, time.Now().Add(RefreshTokenTTL))

	revoked, err := fixture.store.RevokeFamily(ctx, target, time.Now())
	if err != nil {
		t.Fatalf("revoke family: %v", err)
	}
	if revoked != 3 {
		t.Errorf("revoked = %d, want 3", revoked)
	}

	for index, token := range tokens {
		session, err := fixture.store.FindByTokenHash(ctx, HashToken(token))
		if err != nil {
			t.Fatalf("find revoked token %d: %v", index, err)
		}
		if !session.Revoked() {
			t.Errorf("token %d in the revoked family is still usable", index)
		}
	}

	survivor, err := fixture.store.FindByTokenHash(ctx, HashToken(untouched))
	if err != nil {
		t.Fatalf("find the bystander token: %v", err)
	}
	if survivor.Revoked() {
		t.Error("revoking one family revoked another; every logged-in device would be signed out")
	}

	// Idempotent: the WHERE revoked_at IS NULL clause means a second call reports
	// nothing new rather than rewriting the timestamps. Ten seconds later on purpose —
	// with the same timestamp MySQL would report nothing changed regardless of the
	// clause, and the assertion would hold against a store that revokes twice.
	again, err := fixture.store.RevokeFamily(ctx, target, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatalf("revoke family again: %v", err)
	}
	if again != 0 {
		t.Errorf("second revoke reported %d rows, want 0", again)
	}
}

func TestRevokeUserRevokesEveryFamily(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	first := fixture.family("device-a")
	second := fixture.family("device-b")
	fixture.create(t, ctx, first, time.Now().Add(RefreshTokenTTL))
	fixture.create(t, ctx, second, time.Now().Add(RefreshTokenTTL))

	// Scoped by user, so other rows for the same account from other runs would be
	// counted too; assert at least ours rather than an exact number.
	revoked, err := fixture.store.RevokeUser(ctx, fixture.userID, time.Now())
	if err != nil {
		t.Fatalf("revoke user: %v", err)
	}
	if revoked < 2 {
		t.Errorf("revoked = %d, want at least the 2 sessions this test created", revoked)
	}
}

// The unique index on token_hash is what stops one token from authenticating as
// another session. Storing the same hash twice must fail loudly.
func TestDuplicateTokenHashIsRejected(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	family := fixture.family("dupe")
	_, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	record := NewSession{
		UserID:    fixture.userID,
		FamilyID:  family,
		TokenHash: hash,
		CSRFHash:  hash,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
	}
	if _, err := fixture.store.Create(ctx, record); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := fixture.store.Create(ctx, record); err == nil {
		t.Fatal("the same token hash was stored twice; the unique index is missing")
	}
}

// A long User-Agent must not fail the insert. MySQL in strict mode rejects an
// oversized string rather than truncating it, so the store trims in Go; if that
// trim is ever removed, a browser with a verbose UA string cannot log in.
func TestALongUserAgentDoesNotBreakLogin(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	token, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := fixture.store.Create(ctx, NewSession{
		UserID:    fixture.userID,
		FamilyID:  fixture.family("long-ua"),
		TokenHash: hash,
		CSRFHash:  hash,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
		UserAgent: strings.Repeat("Mozilla/5.0 ", 40), // 480 bytes into a VARCHAR(255)
	}); err != nil {
		t.Fatalf("create with a 480 byte user agent: %v", err)
	}
	if _, err := fixture.store.FindByTokenHash(ctx, HashToken(token)); err != nil {
		t.Fatalf("the session was not stored: %v", err)
	}
}

// DeleteExpired is bounded so a cleanup cannot lock the table while it deletes
// months of rows. The limit has to actually apply, or the bound is decorative.
func TestDeleteExpiredHonoursTheLimit(t *testing.T) {
	fixture, ctx := newSessionFixture(t)
	family := fixture.family("purge")
	for range 3 {
		fixture.create(t, ctx, family, time.Now().Add(-48*time.Hour))
	}
	// One that is still live, to prove the cutoff is read rather than ignored.
	_, live := fixture.create(t, ctx, family, time.Now().Add(RefreshTokenTTL))

	// Scoped by the cutoff, not by family, so another run's expired rows could be
	// swept instead of ours. Delete in bounded passes and count until ours are gone.
	deleted, err := fixture.store.DeleteExpired(ctx, time.Now().Add(-24*time.Hour), 2)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if deleted != 2 {
		t.Errorf("first pass deleted %d rows, want exactly the limit of 2", deleted)
	}

	for range 5 {
		more, err := fixture.store.DeleteExpired(ctx, time.Now().Add(-24*time.Hour), 2)
		if err != nil {
			t.Fatalf("delete expired: %v", err)
		}
		if more == 0 {
			break
		}
	}

	// The live session must have survived every pass.
	if _, err := fixture.store.FindByTokenHash(ctx, HashToken(live)); err != nil {
		t.Fatalf("the cleanup deleted a live session: %v", err)
	}
}

// The e-mail comparison is left to MySQL because the column collation is
// utf8mb4_unicode_ci, which is case-insensitive; lower-casing in Go instead would be
// a different rule. This asserts the collation is what the store assumes, using an
// address read from the database rather than a hard-coded one.
func TestUserStoreLooksUpEmailCaseInsensitively(t *testing.T) {
	database := testDatabase(t)
	ctx := context.Background()
	store := NewMySQLUserStore(database)

	var (
		userID int64
		email  string
	)
	if err := database.QueryRowContext(ctx,
		`SELECT id, email FROM users WHERE email REGEXP '[a-z]' ORDER BY id LIMIT 1`,
	).Scan(&userID, &email); err != nil {
		t.Fatalf("find an account to look up: %v", err)
	}

	for _, variant := range []string{email, strings.ToUpper(email), " " + email + " "} {
		// The service trims; the store does not, so the padded variant is looked up
		// exactly as given and is expected to miss.
		credentials, err := store.FindByEmail(ctx, variant)
		if strings.TrimSpace(variant) != variant {
			if err != ErrUserNotFound {
				t.Errorf("a padded address resolved to user %d; trimming belongs to the service, not the store",
					credentials.UserID)
			}
			continue
		}
		if err != nil {
			t.Errorf("look up a case variant: %v", err)
			continue
		}
		if credentials.UserID != userID {
			t.Errorf("case variant resolved to user %d, want %d", credentials.UserID, userID)
		}
	}
}

func TestUserStoreUnknownEmailIsErrUserNotFound(t *testing.T) {
	database := testDatabase(t)
	store := NewMySQLUserStore(database)

	_, err := store.FindByEmail(context.Background(), "definitely-not-registered@invalid.test")
	if err != ErrUserNotFound {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

// The production condition behind the empty-password guard: an account created
// through Google or Twitch has password = ” rather than NULL. The store must return
// it as an empty string so the service can reject it, and must not fail the scan.
//
// If NULL ever appears in that column the scan into a string fails and login breaks
// for that account, so this also pins the column's shape.
func TestUserStoreReportsAPasswordlessAccountAsAnEmptyHash(t *testing.T) {
	database := testDatabase(t)
	ctx := context.Background()
	store := NewMySQLUserStore(database)

	var userID int64
	err := database.QueryRowContext(ctx,
		`SELECT id FROM users WHERE password = '' OR password IS NULL ORDER BY id LIMIT 1`).Scan(&userID)
	if err == sql.ErrNoRows {
		t.Skip("no passwordless account in this database")
	}
	if err != nil {
		t.Fatalf("find a passwordless account: %v", err)
	}

	credentials, err := store.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("read a passwordless account: %v", err)
	}
	if credentials.PasswordHash != "" {
		t.Errorf("password hash = %q, want an empty string", credentials.PasswordHash)
	}
	if credentials.Roles == nil {
		t.Error("roles must be an empty slice rather than nil, so the claim encodes as [] and not null")
	}
}
