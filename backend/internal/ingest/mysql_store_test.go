package ingest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

// The ingest store against the real server. What matters here is that an element and its
// pivot row land together — an element with no pivot belongs to no post, so nothing lists
// it and none of the deletion paths, which all start from a post, would ever find it.

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
		Host: host, Port: port,
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

type fixture struct {
	database  *sql.DB
	store     *MySQLStore
	namespace string
	owner     int64
	stranger  int64
	postID    int64
	serial    string
}

func newFixture(t *testing.T) (*fixture, context.Context) {
	t.Helper()
	database := testDatabase(t)
	ctx := context.Background()

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("namespace: %v", err)
	}
	fixture := &fixture{
		database:  database,
		store:     NewMySQLStore(database),
		namespace: "go-ingest-" + hex.EncodeToString(suffix),
	}
	// This fixture creates posts, which another package's test counts. See
	// sharedlock_test.go.
	lockSharedPosts(t, database)
	t.Cleanup(func() { fixture.cleanUp(t) })

	fixture.owner = fixture.createUser(t, ctx, "owner")
	fixture.stranger = fixture.createUser(t, ctx, "stranger")
	fixture.postID, fixture.serial = fixture.createPost(t, ctx, fixture.owner)
	return fixture, ctx
}

func (fixture *fixture) cleanUp(t *testing.T) {
	t.Helper()
	for _, statement := range []string{
		`DELETE pe FROM post_elements pe JOIN posts p ON p.id = pe.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE pp FROM post_policies pp JOIN posts p ON p.id = pp.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE p FROM posts p JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE FROM users WHERE email LIKE ?`,
	} {
		if _, err := fixture.database.Exec(statement, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up: %v", err)
		}
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM elements WHERE source_url LIKE ?`,
		"https://"+fixture.namespace+"%"); err != nil {
		t.Errorf("clean up elements: %v", err)
	}
}

func (fixture *fixture) createUser(t *testing.T, ctx context.Context, name string) int64 {
	t.Helper()
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO users (name, email, password, created_at, updated_at)
		 VALUES ('go test', ?, '', NOW(), NOW())`,
		fmt.Sprintf("%s-%s@invalid.test", fixture.namespace, name))
	if err != nil {
		t.Fatalf("create a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("user id: %v", err)
	}
	return userID
}

func (fixture *fixture) createPost(t *testing.T, ctx context.Context, userID int64) (int64, string) {
	t.Helper()
	serial := fixture.namespace[len(fixture.namespace)-8:]
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO posts (user_id, serial, title, description, created_at, updated_at)
		 VALUES (?, ?, 'a title', 'a description', NOW(), NOW())`, userID, serial)
	if err != nil {
		t.Fatalf("create a post: %v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("post id: %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_policies (post_id, access_policy, created_at, updated_at)
		 VALUES (?, 'public', NOW(), NOW())`, postID); err != nil {
		t.Fatalf("create a policy: %v", err)
	}
	return postID, serial
}

func (fixture *fixture) newElement(kind string) NewElement {
	element := NewElement{
		PostID:    fixture.postID,
		SourceURL: "https://" + fixture.namespace + "/media." + kind,
		ThumbURL:  "https://" + fixture.namespace + "/media." + kind,
		Title:     "an element",
		Type:      TypeImage,
	}
	if kind == "mp4" {
		element.Type = TypeVideo
		element.VideoSource = VideoSourceFile
	}
	return element
}

func TestPostForOwnerCountsWhatIsAlreadyAttached(t *testing.T) {
	fixture, ctx := newFixture(t)

	postID, elements, err := fixture.store.PostForOwner(ctx, fixture.owner, fixture.serial)
	if err != nil {
		t.Fatalf("PostForOwner() error = %v", err)
	}
	if postID != fixture.postID {
		t.Errorf("post = %d, want %d", postID, fixture.postID)
	}
	if elements != 0 {
		t.Errorf("elements = %d, want 0", elements)
	}

	if _, err := fixture.store.CreateElement(ctx, fixture.newElement("png")); err != nil {
		t.Fatalf("CreateElement() error = %v", err)
	}
	if _, elements, _ = fixture.store.PostForOwner(ctx, fixture.owner, fixture.serial); elements != 1 {
		t.Errorf("elements = %d, want 1", elements)
	}
}

// A soft-deleted element does not count against the post's cap: the author removed it, and
// the row only lingers so history keeps resolving.
func TestADeletedElementDoesNotCountTowardsTheCap(t *testing.T) {
	fixture, ctx := newFixture(t)
	stored, err := fixture.store.CreateElement(ctx, fixture.newElement("png"))
	if err != nil {
		t.Fatalf("CreateElement() error = %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE elements SET deleted_at = NOW() WHERE id = ?`, stored.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	_, elements, err := fixture.store.PostForOwner(ctx, fixture.owner, fixture.serial)
	if err != nil {
		t.Fatalf("PostForOwner() error = %v", err)
	}
	if elements != 0 {
		t.Errorf("elements = %d, want 0", elements)
	}
}

// Ownership is in the statement, as everywhere else in the editor.
func TestAStrangerCannotResolveThePost(t *testing.T) {
	fixture, ctx := newFixture(t)

	if _, _, err := fixture.store.PostForOwner(ctx, fixture.stranger, fixture.serial); err != ErrPostNotFound {
		t.Fatalf("error = %v, want ErrPostNotFound", err)
	}
}

func TestCreateElementWritesTheRowAndThePivotTogether(t *testing.T) {
	fixture, ctx := newFixture(t)

	stored, err := fixture.store.CreateElement(ctx, fixture.newElement("mp4"))
	if err != nil {
		t.Fatalf("CreateElement() error = %v", err)
	}

	var (
		attached    int
		elementType string
		videoSource sql.NullString
		path        sql.NullString
	)
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM post_elements WHERE element_id = e.id),
		        e.type, e.video_source, e.path
		   FROM elements AS e WHERE e.id = ?`, stored.ID).
		Scan(&attached, &elementType, &videoSource, &path); err != nil {
		t.Fatalf("read the element back: %v", err)
	}
	if attached != 1 {
		t.Errorf("the element is attached to %d posts, want 1", attached)
	}
	if elementType != TypeVideo {
		t.Errorf("type = %q, want video", elementType)
	}
	if videoSource.String != VideoSourceFile {
		t.Errorf("video_source = %q, want %q", videoSource.String, VideoSourceFile)
	}
	// An element with no stored copy holds NULL rather than "", which is what the
	// 158,969 youtube rows hold.
	if path.Valid {
		t.Errorf("path = %q, want NULL when nothing was stored", path.String)
	}
}

// A pivot that cannot be written must take the element with it, or the row is orphaned:
// nothing lists it, and every deletion path starts from a post.
func TestAFailedAttachLeavesNoOrphanElement(t *testing.T) {
	fixture, ctx := newFixture(t)
	element := fixture.newElement("png")
	// A post id that does not exist, which the pivot's foreign key refuses.
	element.PostID = -1

	if _, err := fixture.store.CreateElement(ctx, element); err == nil {
		t.Fatal("CreateElement() accepted a post that does not exist")
	}

	var orphans int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM elements WHERE source_url = ?`, element.SourceURL).Scan(&orphans); err != nil {
		t.Fatalf("count orphans: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d elements were left with no post", orphans)
	}
}
