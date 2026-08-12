package postaccess

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

/*
The store, against the real schema.

Three things here can only be checked against MySQL: that the join reaches the policy row
at all, that a soft-deleted post is gone, and that the column's nullability is handled. The
last is not hypothetical — this database holds 1,035 password posts and one of them has a
NULL digest, which must open for nobody rather than for anyone who sends nothing.
*/

func storeTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MYSQL_TEST_HOST is not set; skipping MySQL integration test")
	}
	port, err := strconv.Atoi(storeEnvOr("MYSQL_TEST_PORT", "3306"))
	if err != nil {
		t.Fatalf("MYSQL_TEST_PORT is not a number: %v", err)
	}
	database, err := mysqlstore.Open(config.DatabaseConfig{
		Host: host, Port: port,
		Name:            storeEnvOr("MYSQL_TEST_DATABASE", "rk_db_restore_20260729"),
		User:            storeEnvOr("MYSQL_TEST_USERNAME", "root"),
		Password:        os.Getenv("MYSQL_TEST_PASSWORD"),
		MaxOpenConns:    4,
		MaxIdleConns:    2,
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

func storeEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type storeFixture struct {
	database  *sql.DB
	store     *MySQLStore
	namespace string
	owner     int64
}

func newStoreFixture(t *testing.T) (*storeFixture, context.Context) {
	t.Helper()
	database := storeTestDatabase(t)
	ctx := context.Background()

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("namespace: %v", err)
	}
	fixture := &storeFixture{
		database:  database,
		store:     NewMySQLStore(database),
		namespace: "go-postaccess-" + hex.EncodeToString(suffix),
	}
	// This fixture creates posts, which another package's test counts. See
	// sharedlock_test.go.
	lockSharedPosts(t, database)
	t.Cleanup(func() { fixture.cleanUp(t) })

	result, err := database.ExecContext(ctx,
		`INSERT INTO users (name, email, password, created_at, updated_at)
		 VALUES ('go test', ?, '', NOW(), NOW())`, fixture.namespace+"@invalid.test")
	if err != nil {
		t.Fatalf("create a user: %v", err)
	}
	if fixture.owner, err = result.LastInsertId(); err != nil {
		t.Fatalf("user id: %v", err)
	}
	return fixture, ctx
}

func (fixture *storeFixture) cleanUp(t *testing.T) {
	t.Helper()
	for _, statement := range []string{
		`DELETE pp FROM post_policies pp JOIN posts p ON p.id = pp.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE p FROM posts p JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE FROM users WHERE email LIKE ?`,
	} {
		if _, err := fixture.database.Exec(statement, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up: %v", err)
		}
	}
}

// addPost writes one post. A nil digest is the real NULL this column allows.
func (fixture *storeFixture) addPost(
	t *testing.T, ctx context.Context, policy string, digest any, deleted bool,
) string {
	t.Helper()
	serialBytes := make([]byte, 4)
	if _, err := rand.Read(serialBytes); err != nil {
		t.Fatalf("serial: %v", err)
	}
	serial := hex.EncodeToString(serialBytes)

	deletedAt := "NULL"
	if deleted {
		deletedAt = "NOW()"
	}
	result, err := fixture.database.ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO posts (user_id, serial, title, description, deleted_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'a description', %s, NOW(), NOW())`, deletedAt),
		fixture.owner, serial, fixture.namespace)
	if err != nil {
		t.Fatalf("create a post: %v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("post id: %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_policies (post_id, access_policy, password, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())`, postID, policy, digest); err != nil {
		t.Fatalf("create a policy: %v", err)
	}
	return serial
}

func TestTheStoreReadsAPostsPolicyAndDigest(t *testing.T) {
	fixture, ctx := newStoreFixture(t)
	serial := fixture.addPost(t, ctx, PolicyPassword, HashPassword("door-code"), false)

	post, err := fixture.store.Post(ctx, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if post.Serial != serial || post.OwnerID != fixture.owner || !post.RequiresPassword() {
		t.Fatalf("post = %#v", post)
	}
	if !PasswordMatches("door-code", post.PasswordDigest) {
		t.Error("the digest read back does not match the password that produced it")
	}
}

func TestASoftDeletedPostIsNotFound(t *testing.T) {
	fixture, ctx := newStoreFixture(t)
	serial := fixture.addPost(t, ctx, PolicyPassword, HashPassword("door-code"), true)

	if _, err := fixture.store.Post(ctx, serial); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("Post() error = %v, want ErrPostNotFound", err)
	}
}

/*
A NULL DIGEST OPENS FOR NOBODY.

This database has one password post whose password column is NULL. Scanning NULL into a
string would fail outright, and treating it as the empty string and then comparing would
let anyone in — so the column is COALESCEd and PasswordMatches refuses an empty stored
digest. Laravel's `hash(...) === $policy->password` refused it the same way.
*/
func TestAPasswordPostWithNoDigestCannotBeUnlocked(t *testing.T) {
	fixture, ctx := newStoreFixture(t)
	serial := fixture.addPost(t, ctx, PolicyPassword, nil, false)

	post, err := fixture.store.Post(ctx, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if post.PasswordDigest != "" {
		t.Fatalf("digest = %q, want the empty string", post.PasswordDigest)
	}

	signer, err := NewSigner([]byte("a-deployment-secret"))
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}
	service, err := NewService(ServiceOptions{Store: fixture.store, Signer: signer})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	for _, guess := range []string{"", "door-code"} {
		if _, _, err := service.Grant(ctx, serial, guess); !errors.Is(err, ErrWrongPassword) {
			t.Errorf("Grant(%q) error = %v, want ErrWrongPassword", guess, err)
		}
	}
}

func TestAnUnknownSerialIsNotFoundInTheDatabase(t *testing.T) {
	fixture, ctx := newStoreFixture(t)

	if _, err := fixture.store.Post(ctx, "no-such-post"); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("Post() error = %v, want ErrPostNotFound", err)
	}
}
