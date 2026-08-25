package comments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/platform/mysqlstore"
)

// The floor numbering, the depth limit and the tombstone rules are all expressed in SQL
// against the real schema, so they are tested against a real database or not at all.

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

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// testPost creates a public post of its own, so the suite never comments on real ones.
func testPost(t *testing.T, database *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	serial := fmt.Sprintf("gotest-%d", time.Now().UnixNano())
	result, err := database.ExecContext(ctx,
		`INSERT INTO posts (serial, created_at, updated_at) VALUES (?, NOW(), NOW())`, serial)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("post id: %v", err)
	}
	if _, err := database.ExecContext(ctx,
		`INSERT INTO post_policies (post_id, access_policy, created_at, updated_at)
		 VALUES (?, 'public', NOW(), NOW())`, postID); err != nil {
		t.Fatalf("create post policy: %v", err)
	}
	t.Cleanup(func() {
		clean := context.Background()
		// The comments go first: post_comments cascades from the post, but the comments
		// themselves are only reachable through it.
		if _, err := database.ExecContext(clean,
			`DELETE c FROM comments c JOIN post_comments pc ON pc.comment_id = c.id WHERE pc.post_id = ?`, postID); err != nil {
			t.Errorf("clean comments: %v", err)
		}
		if _, err := database.ExecContext(clean, `DELETE FROM post_policies WHERE post_id = ?`, postID); err != nil {
			t.Errorf("clean post policy: %v", err)
		}
		if _, err := database.ExecContext(clean, `DELETE FROM posts WHERE id = ?`, postID); err != nil {
			t.Errorf("clean post: %v", err)
		}
	})
	return serial
}

// guest is a signed-out commenter holding a delete key.
func guest(key string) Viewer {
	return Viewer{AnonymousID: "browser-" + key, DeleteHash: key}
}

// say posts a comment from its own IP, because the create rate limit counts three a
// minute per address and these tests post more than three.
func say(t *testing.T, repository *MySQLRepository, serial, content string, parentID *int64, viewer Viewer, address int) Comment {
	t.Helper()
	comment, err := repository.Create(context.Background(), serial, CreateInput{
		Content: content, AnonymousID: viewer.AnonymousID, ParentID: parentID,
		IP: fmt.Sprintf("10.0.0.%d", address), Viewer: viewer,
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", content, err)
	}
	return comment
}

func contents(items []Comment) []string {
	said := make([]string, 0, len(items))
	for _, item := range items {
		if item.Deleted {
			said = append(said, "[deleted]")
			continue
		}
		said = append(said, item.Content)
	}
	return said
}

func TestFloorsAreNumberedFromTheOldestAndRepliesNestThreeDeep(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	serial := testPost(t, database)
	viewer := guest("key-a")

	first := say(t, repository, serial, "first floor", nil, viewer, 1)
	second := say(t, repository, serial, "second floor", nil, viewer, 2)
	reply := say(t, repository, serial, "a reply", &first.ID, viewer, 3)
	deep := say(t, repository, serial, "a reply to the reply", &reply.ID, viewer, 4)

	if first.Floor == nil || *first.Floor != 1 || second.Floor == nil || *second.Floor != 2 {
		t.Fatalf("floors = %v / %v", first.Floor, second.Floor)
	}
	if reply.Floor != nil || reply.Depth != 2 || deep.Depth != 3 {
		t.Fatalf("reply = %#v, deep = %#v", reply, deep)
	}
	if _, err := repository.Create(context.Background(), serial, CreateInput{
		Content: "one level too far", ParentID: &deep.ID, IP: "10.0.0.5", Viewer: viewer,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("a fourth level was accepted: err = %v", err)
	}

	page, err := repository.List(context.Background(), serial, 1, 10, viewer)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	// Newest floor first, and each floor immediately followed by its own thread,
	// oldest reply first.
	want := []string{"second floor", "first floor", "a reply", "a reply to the reply"}
	if got := contents(page.Items); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	if page.Total != 4 || page.TotalPages != 1 {
		t.Fatalf("total = %d, pages = %d", page.Total, page.TotalPages)
	}
	if page.Items[0].Floor == nil || *page.Items[0].Floor != 2 || page.Items[2].Floor != nil {
		t.Fatalf("floors = %#v", page.Items)
	}
}

func TestADeletedFloorKeepsItsNumberAndTheThreadUnderIt(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	serial := testPost(t, database)
	viewer := guest("key-a")

	first := say(t, repository, serial, "first floor", nil, viewer, 11)
	say(t, repository, serial, "second floor", nil, viewer, 12)
	say(t, repository, serial, "a reply", &first.ID, viewer, 13)

	if err := repository.Delete(context.Background(), serial, first.ID, viewer); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	page, err := repository.List(context.Background(), serial, 1, 10, viewer)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []string{"second floor", "[deleted]", "a reply"}
	if got := contents(page.Items); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	tombstone := page.Items[1]
	if tombstone.Floor == nil || *tombstone.Floor != 1 {
		t.Fatalf("the floor number was lost: %#v", tombstone)
	}
	// Nothing about the author or the text survives the deletion.
	if tombstone.Nickname != "" || tombstone.Content != "" || tombstone.AvatarURL != nil ||
		len(tombstone.Champions) != 0 || tombstone.CanDelete {
		t.Fatalf("tombstone = %#v", tombstone)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want the two readable comments", page.Total)
	}
	// A new comment lands on the next floor: the deleted one still occupies its own.
	third := say(t, repository, serial, "third floor", nil, viewer, 14)
	if third.Floor == nil || *third.Floor != 3 {
		t.Fatalf("floor = %v, want 3", third.Floor)
	}
}

func TestADeletedReplyIsDroppedUnlessSomethingUnderItSurvives(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	serial := testPost(t, database)
	viewer := guest("key-a")

	floor := say(t, repository, serial, "the floor", nil, viewer, 21)
	reply := say(t, repository, serial, "a reply", &floor.ID, viewer, 22)
	deep := say(t, repository, serial, "a reply to the reply", &reply.ID, viewer, 23)

	// Deleted, but it is holding up a reply that is still there.
	if err := repository.Delete(context.Background(), serial, reply.ID, viewer); err != nil {
		t.Fatalf("Delete(reply) error = %v", err)
	}
	page, err := repository.List(context.Background(), serial, 1, 10, viewer)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []string{"the floor", "[deleted]", "a reply to the reply"}
	if got := contents(page.Items); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}

	// With nothing left underneath, the tombstone has no shape to hold and goes.
	if err := repository.Delete(context.Background(), serial, deep.ID, viewer); err != nil {
		t.Fatalf("Delete(deep) error = %v", err)
	}
	page, err = repository.List(context.Background(), serial, 1, 10, viewer)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := contents(page.Items); fmt.Sprint(got) != fmt.Sprint([]string{"the floor"}) {
		t.Fatalf("items = %v, want just the floor", got)
	}
	// Replying to something deleted is not a thing.
	if _, err := repository.Create(context.Background(), serial, CreateInput{
		Content: "hello?", ParentID: &reply.ID, IP: "10.0.0.24", Viewer: viewer,
	}); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("a deleted comment was replied to: err = %v", err)
	}
}

func TestOnlyTheBrowserOrAccountThatWroteACommentMayDeleteIt(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	serial := testPost(t, database)
	author := guest("key-a")
	stranger := guest("key-b")

	comment := say(t, repository, serial, "mine", nil, author, 31)

	if err := repository.Delete(context.Background(), serial, comment.ID, stranger); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another browser deleted it: err = %v", err)
	}
	if err := repository.Delete(context.Background(), serial, comment.ID, Viewer{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a browser with no key deleted it: err = %v", err)
	}

	seen, err := repository.List(context.Background(), serial, 1, 10, stranger)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if seen.Items[0].CanDelete {
		t.Fatalf("a stranger was offered the delete: %#v", seen.Items[0])
	}
	seen, err = repository.List(context.Background(), serial, 1, 10, author)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !seen.Items[0].CanDelete {
		t.Fatalf("the author was not offered the delete: %#v", seen.Items[0])
	}

	if err := repository.Delete(context.Background(), serial, comment.ID, author); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	// Twice is once too many: the second attempt cannot tell the caller anything the
	// first did not.
	if err := repository.Delete(context.Background(), serial, comment.ID, author); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting twice = %v", err)
	}
}

func TestAnAccountOwnsItsCommentEvenFromAnotherBrowser(t *testing.T) {
	database := testDatabase(t)
	repository := NewMySQLRepository(database)
	serial := testPost(t, database)

	var userID int64
	if err := database.QueryRowContext(context.Background(),
		`SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Skipf("no user to comment as: %v", err)
	}
	account := Viewer{UserID: &userID, AnonymousID: "browser-a", DeleteHash: "key-a"}
	comment := say(t, repository, serial, "signed in", nil, account, 41)

	// The browser that posted it, now signed out, is not the owner: the account is.
	if err := repository.Delete(context.Background(), serial, comment.ID, guest("key-a")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the cookie outranked the account: err = %v", err)
	}
	elsewhere := Viewer{UserID: &userID, AnonymousID: "browser-z", DeleteHash: "key-z"}
	if err := repository.Delete(context.Background(), serial, comment.ID, elsewhere); err != nil {
		t.Fatalf("Delete() from another browser error = %v", err)
	}
}
