package authoring

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

// The editor's SQL against the real server, on throwaway rows in the restore database.
//
// What is worth proving here is not that a SELECT returns rows. It is that ownership
// rides inside every statement — a serial belonging to someone else must not resolve, in
// any of the eight — and that deleting a post leaves the database in the state the three
// Laravel effects left it in, all or nothing.

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
	database     *sql.DB
	repository   *MySQLRepository
	namespace    string
	tagNamespace string
	owner        int64
	stranger     int64
}

// tag builds a fixture tag name that fits the column.
func (fixture *fixture) tag(suffix string) string {
	return fixture.tagNamespace + suffix
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
		database:   database,
		repository: NewMySQLRepository(database),
		namespace:  "go-editor-" + hex.EncodeToString(suffix),
		// tags.name is VARCHAR(15), so the tag fixtures need a namespace that leaves
		// room for a suffix inside it rather than the long one used for e-mail addresses.
		tagNamespace: "t" + hex.EncodeToString(suffix),
	}

	// This fixture creates posts, which another package's test counts. See
	// sharedlock_test.go.
	lockSharedPosts(t, database)
	t.Cleanup(func() { fixture.cleanUp(t) })

	fixture.owner = fixture.createUser(t, ctx, "owner")
	fixture.stranger = fixture.createUser(t, ctx, "stranger")
	return fixture, ctx
}

// cleanUp removes everything the fixture created, children first.
func (fixture *fixture) cleanUp(t *testing.T) {
	t.Helper()
	statements := []string{
		`DELETE rr FROM rank_reports rr JOIN posts p ON p.id = rr.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE pe FROM post_elements pe JOIN posts p ON p.id = pe.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE pt FROM post_tags pt JOIN posts p ON p.id = pt.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE ps FROM post_statistics ps JOIN posts p ON p.id = ps.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE pp FROM post_policies pp JOIN posts p ON p.id = pp.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE p FROM posts p JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE FROM users WHERE email LIKE ?`,
	}
	for _, statement := range statements {
		if _, err := fixture.database.Exec(statement, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up: %v", err)
		}
	}
	if _, err := fixture.database.Exec(
		`DELETE FROM tags WHERE name LIKE ?`, fixture.tagNamespace+"%"); err != nil {
		t.Errorf("clean up tags: %v", err)
	}
	// The elements are not reachable from the posts once the pivot is gone, so they are
	// tracked by their source_url instead.
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
		 VALUES (?, ?, '', NOW(), NOW())`,
		"go test", fmt.Sprintf("%s-%s@invalid.test", fixture.namespace, name))
	if err != nil {
		t.Fatalf("create a user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("user id: %v", err)
	}
	return userID
}

// createPost writes a post directly, for the read tests.
func (fixture *fixture) createPost(t *testing.T, ctx context.Context, userID int64, title string) (int64, string) {
	t.Helper()
	serial, err := NewPostSerial()
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO posts (user_id, serial, title, description, created_at, updated_at)
		 VALUES (?, ?, ?, 'a description', NOW(), NOW())`, userID, serial, title)
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

// addElement attaches a new element to a post and answers with its id.
func (fixture *fixture) addElement(t *testing.T, ctx context.Context, postID int64, title string) int64 {
	t.Helper()
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO elements (source_url, thumb_url, title, type, created_at, updated_at)
		 VALUES (?, ?, ?, 'image', NOW(), NOW())`,
		"https://"+fixture.namespace+"/"+title, "https://"+fixture.namespace+"/thumb", title)
	if err != nil {
		t.Fatalf("create an element: %v", err)
	}
	elementID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("element id: %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_elements (post_id, element_id) VALUES (?, ?)`, postID, elementID); err != nil {
		t.Fatalf("attach the element: %v", err)
	}
	return elementID
}

func (fixture *fixture) addRankReport(t *testing.T, ctx context.Context, postID, elementID int64, rank int) {
	t.Helper()
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO rank_reports (post_id, element_id, `+"`rank`"+`, win_rate, final_win_rate,
		                           created_at, updated_at)
		 VALUES (?, ?, ?, 62.50, 75.00, NOW(), NOW())`, postID, elementID, rank); err != nil {
		t.Fatalf("create a rank report: %v", err)
	}
}

func (fixture *fixture) count(t *testing.T, ctx context.Context, query string, arguments ...any) int {
	t.Helper()
	var count int
	if err := fixture.database.QueryRowContext(ctx, query, arguments...).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func TestCreatePostWritesThePostItsPolicyAndItsTags(t *testing.T) {
	fixture, ctx := newFixture(t)
	serial, err := NewPostSerial()
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	draft := PostDraft{
		Title: "a title", Description: "a description", AccessPolicy: PolicyPassword,
		Tags: []string{fixture.tag("cats"), fixture.tag("dogs")},
	}
	if err := fixture.repository.CreatePost(ctx, fixture.owner, serial, draft, "a-hash"); err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}

	post, err := fixture.repository.Post(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if post.Title != "a title" || post.AccessPolicy != PolicyPassword {
		t.Errorf("post = %+v", post)
	}
	if !post.HasPassword {
		t.Error("HasPassword = false although a hash was written")
	}
	if len(post.Tags) != 2 {
		t.Errorf("tags = %v, want two", post.Tags)
	}
}

// A post with no password holds NULL, not "": 5,166 production posts are in that state
// and the read path tests for both.
func TestAPostWithoutAPasswordStoresNull(t *testing.T) {
	fixture, ctx := newFixture(t)
	serial, _ := NewPostSerial()

	if err := fixture.repository.CreatePost(ctx, fixture.owner, serial,
		PostDraft{Title: "t", Description: "d", AccessPolicy: PolicyPublic}, ""); err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}

	var password sql.NullString
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT pp.password FROM post_policies pp JOIN posts p ON p.id = pp.post_id
		  WHERE p.serial = ?`, serial).Scan(&password); err != nil {
		t.Fatalf("read the password: %v", err)
	}
	if password.Valid {
		t.Errorf("password = %q, want NULL", password.String)
	}
}

// THE OWNERSHIP TEST, ACROSS EVERY STATEMENT. Reading, editing or deleting someone
// else's post has to answer "not found" — not "forbidden", which would confirm the serial
// exists, and certainly not the row.
func TestAStrangerSeesNothingAndChangesNothing(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "the owner's post")
	elementID := fixture.addElement(t, ctx, postID, "an element")

	if _, err := fixture.repository.Post(ctx, fixture.stranger, serial); err != ErrPostNotFound {
		t.Errorf("Post() error = %v, want ErrPostNotFound", err)
	}
	if _, err := fixture.repository.Elements(ctx, fixture.stranger, serial, ElementQuery{}.Normalized()); err != ErrPostNotFound {
		t.Errorf("Elements() error = %v, want ErrPostNotFound", err)
	}
	if err := fixture.repository.UpdatePost(ctx, fixture.stranger, serial,
		PostDraft{Title: "stolen", Description: "d", AccessPolicy: PolicyPublic}, nil); err != ErrPostNotFound {
		t.Errorf("UpdatePost() error = %v, want ErrPostNotFound", err)
	}
	if _, err := fixture.repository.DeletePost(ctx, fixture.stranger, serial); err != ErrPostNotFound {
		t.Errorf("DeletePost() error = %v, want ErrPostNotFound", err)
	}
	title := "stolen"
	if _, err := fixture.repository.UpdateElement(ctx, fixture.stranger, elementID,
		ElementEdit{Title: &title}); err != ErrElementNotFound {
		t.Errorf("UpdateElement() error = %v, want ErrElementNotFound", err)
	}
	if _, err := fixture.repository.DeleteElement(ctx, fixture.stranger, elementID); err != ErrElementNotFound {
		t.Errorf("DeleteElement() error = %v, want ErrElementNotFound", err)
	}

	// And nothing moved.
	post, err := fixture.repository.Post(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if post.Title != "the owner's post" {
		t.Errorf("title = %q; a stranger changed it", post.Title)
	}
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM elements WHERE id = ? AND deleted_at IS NULL`, elementID) != 1 {
		t.Error("a stranger deleted the element")
	}
}

func TestListPostsIsScopedToTheOwnerAndPaged(t *testing.T) {
	fixture, ctx := newFixture(t)
	for index := 0; index < 3; index++ {
		fixture.createPost(t, ctx, fixture.owner, fmt.Sprintf("post %d", index))
	}
	fixture.createPost(t, ctx, fixture.stranger, "not yours")

	posts, total, err := fixture.repository.ListPosts(ctx, fixture.owner, 1, 2)
	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 — the stranger's post must not be counted", total)
	}
	if len(posts) != 2 {
		t.Errorf("page held %d posts, want 2", len(posts))
	}

	second, _, err := fixture.repository.ListPosts(ctx, fixture.owner, 2, 2)
	if err != nil {
		t.Fatalf("ListPosts() page 2 error = %v", err)
	}
	if len(second) != 1 {
		t.Errorf("page two held %d posts, want 1", len(second))
	}
	// Newest first, so page two holds the oldest.
	if second[0].Title != "post 0" {
		t.Errorf("page two = %q, want the oldest post", second[0].Title)
	}
}

func TestASoftDeletedPostIsGoneFromTheEditor(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "doomed")
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE posts SET deleted_at = NOW() WHERE id = ?`, postID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, err := fixture.repository.Post(ctx, fixture.owner, serial); err != ErrPostNotFound {
		t.Errorf("Post() error = %v, want ErrPostNotFound", err)
	}
	_, total, err := fixture.repository.ListPosts(ctx, fixture.owner, 1, 15)
	if err != nil {
		t.Fatalf("ListPosts() error = %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

// Deleting a post is three effects in the original — the tags detach, the post and its
// elements soft-delete, the rank reports go — and this is where they either all land or
// none do.
func TestDeletePostRemovesEverythingTheListenersRemoved(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "doomed")
	elementID := fixture.addElement(t, ctx, postID, "an element")
	fixture.addRankReport(t, ctx, postID, elementID, 1)
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT IGNORE INTO tags (name, created_at, updated_at) VALUES (?, NOW(), NOW())`,
		fixture.tag("x")); err != nil {
		t.Fatalf("create a tag: %v", err)
	}
	var tagID int64
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ?`, fixture.tag("x")).Scan(&tagID); err != nil {
		t.Fatalf("read the tag: %v", err)
	}
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_tags (post_id, tag_id, created_at, updated_at) VALUES (?, ?, NOW(), NOW())`,
		postID, tagID); err != nil {
		t.Fatalf("attach the tag: %v", err)
	}

	deletedID, err := fixture.repository.DeletePost(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("DeletePost() error = %v", err)
	}
	if deletedID != postID {
		t.Errorf("deleted id = %d, want %d — the caller queues the rank rebuild with it", deletedID, postID)
	}

	if fixture.count(t, ctx, `SELECT COUNT(*) FROM posts WHERE id = ? AND deleted_at IS NULL`, postID) != 0 {
		t.Error("the post was not soft-deleted")
	}
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM elements WHERE id = ? AND deleted_at IS NULL`, elementID) != 0 {
		t.Error("the post's element was left alive")
	}
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM rank_reports WHERE post_id = ?`, postID) != 0 {
		t.Error("the rank reports were left behind")
	}
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM post_tags WHERE post_id = ?`, postID) != 0 {
		t.Error("the tags were left attached")
	}
	// Soft-deleted, not erased: the rows are still there for anything that reads history.
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM posts WHERE id = ?`, postID) != 1 {
		t.Error("the post row was hard-deleted")
	}
}

// A post is an author's, an element can be in several posts, and deleting one from one
// post must not disturb the other.
func TestDeleteElementDetachesItAndClearsOnlyItsOwnReports(t *testing.T) {
	fixture, ctx := newFixture(t)
	firstPost, _ := fixture.createPost(t, ctx, fixture.owner, "first")
	secondPost, secondSerial := fixture.createPost(t, ctx, fixture.owner, "second")

	shared := fixture.addElement(t, ctx, firstPost, "shared")
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_elements (post_id, element_id) VALUES (?, ?)`, secondPost, shared); err != nil {
		t.Fatalf("attach to the second post: %v", err)
	}
	other := fixture.addElement(t, ctx, secondPost, "other")
	fixture.addRankReport(t, ctx, firstPost, shared, 1)
	fixture.addRankReport(t, ctx, secondPost, shared, 2)
	fixture.addRankReport(t, ctx, secondPost, other, 1)

	affected, err := fixture.repository.DeleteElement(ctx, fixture.owner, shared)
	if err != nil {
		t.Fatalf("DeleteElement() error = %v", err)
	}
	if len(affected) != 2 {
		t.Errorf("affected = %v, want both posts — each one's report needs rebuilding", affected)
	}

	if fixture.count(t, ctx, `SELECT COUNT(*) FROM post_elements WHERE element_id = ?`, shared) != 0 {
		t.Error("the element is still attached to a post")
	}
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM rank_reports WHERE element_id = ?`, shared) != 0 {
		t.Error("the element's rank reports were left behind")
	}
	// The other element in the second post is untouched.
	if fixture.count(t, ctx, `SELECT COUNT(*) FROM rank_reports WHERE element_id = ?`, other) != 1 {
		t.Error("another element's rank report was deleted")
	}
	page, err := fixture.repository.Elements(ctx, fixture.owner, secondSerial, ElementQuery{}.Normalized())
	if err != nil {
		t.Fatalf("Elements() error = %v", err)
	}
	if page.Total != 1 {
		t.Errorf("the second post holds %d elements, want 1", page.Total)
	}
}

func TestElementsCarryTheirRankInThisPostOnly(t *testing.T) {
	fixture, ctx := newFixture(t)
	firstPost, firstSerial := fixture.createPost(t, ctx, fixture.owner, "first")
	secondPost, _ := fixture.createPost(t, ctx, fixture.owner, "second")
	shared := fixture.addElement(t, ctx, firstPost, "shared")
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_elements (post_id, element_id) VALUES (?, ?)`, secondPost, shared); err != nil {
		t.Fatalf("attach to the second post: %v", err)
	}
	fixture.addRankReport(t, ctx, firstPost, shared, 3)
	fixture.addRankReport(t, ctx, secondPost, shared, 9)

	page, err := fixture.repository.Elements(ctx, fixture.owner, firstSerial, ElementQuery{}.Normalized())
	if err != nil {
		t.Fatalf("Elements() error = %v", err)
	}
	if len(page.Elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(page.Elements))
	}
	if page.Elements[0].Rank == nil {
		t.Fatal("the element carries no rank")
	}
	if page.Elements[0].Rank.Rank != 3 {
		t.Errorf("rank = %d, want 3 — the rank from the OTHER post leaked in",
			page.Elements[0].Rank.Rank)
	}
}

func TestAnElementNobodyHasVotedOnHasNoRank(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "fresh")
	fixture.addElement(t, ctx, postID, "unvoted")

	page, err := fixture.repository.Elements(ctx, fixture.owner, serial, ElementQuery{}.Normalized())
	if err != nil {
		t.Fatalf("Elements() error = %v", err)
	}
	if page.Elements[0].Rank != nil {
		t.Errorf("rank = %+v, want none", page.Elements[0].Rank)
	}
}

// The wildcards belong to the query, not to the caller: searching for "100%" is a search
// for that text, not for everything starting with "100".
func TestTheTitleFilterTreatsWildcardsAsText(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "filtered")
	fixture.addElement(t, ctx, postID, "100% cotton")
	fixture.addElement(t, ctx, postID, "100 metres")

	page, err := fixture.repository.Elements(ctx, fixture.owner, serial,
		ElementQuery{TitleLike: "100%"}.Normalized())
	if err != nil {
		t.Fatalf("Elements() error = %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("matched %d elements, want 1 — the %% was treated as a wildcard", page.Total)
	}
	if page.Elements[0].Title != "100% cotton" {
		t.Errorf("matched %q", page.Elements[0].Title)
	}
}

func TestUpdateElementWritesOnlyWhatWasSent(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, _ := fixture.createPost(t, ctx, fixture.owner, "post")
	elementID := fixture.addElement(t, ctx, postID, "before")
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE elements SET video_start_second = 5, video_end_second = 30 WHERE id = ?`,
		elementID); err != nil {
		t.Fatalf("seed the trim: %v", err)
	}

	title := "after"
	element, err := fixture.repository.UpdateElement(ctx, fixture.owner, elementID,
		ElementEdit{Title: &title})
	if err != nil {
		t.Fatalf("UpdateElement() error = %v", err)
	}
	if element.Title != "after" {
		t.Errorf("title = %q, want %q", element.Title, "after")
	}
	if element.StartSecond == nil || *element.StartSecond != 5 {
		t.Errorf("start = %v, want the stored 5 left alone", element.StartSecond)
	}
	if element.EndSecond == nil || *element.EndSecond != 30 {
		t.Errorf("end = %v, want the stored 30 left alone", element.EndSecond)
	}
}

func TestUpdatePostReplacesItsTags(t *testing.T) {
	fixture, ctx := newFixture(t)
	_, serial := fixture.createPost(t, ctx, fixture.owner, "tagged")

	first := PostDraft{Title: "t", Description: "d", AccessPolicy: PolicyPublic,
		Tags: []string{fixture.tag("a"), fixture.tag("b")}}
	if err := fixture.repository.UpdatePost(ctx, fixture.owner, serial, first, nil); err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	second := first
	second.Tags = []string{fixture.tag("b"), fixture.tag("c")}
	if err := fixture.repository.UpdatePost(ctx, fixture.owner, serial, second, nil); err != nil {
		t.Fatalf("second UpdatePost() error = %v", err)
	}

	post, err := fixture.repository.Post(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if len(post.Tags) != 2 {
		t.Fatalf("tags = %v, want two", post.Tags)
	}
	for _, tag := range post.Tags {
		if tag == fixture.tag("a") {
			t.Errorf("tags = %v; the replaced tag is still attached", post.Tags)
		}
	}
}

// Tags nil means "leave them", which is how the original behaved: its rule was
// `sometimes` and syncTags only ran with what the request carried.
func TestUpdatePostWithoutTagsLeavesThemAlone(t *testing.T) {
	fixture, ctx := newFixture(t)
	_, serial := fixture.createPost(t, ctx, fixture.owner, "tagged")

	withTags := PostDraft{Title: "t", Description: "d", AccessPolicy: PolicyPublic,
		Tags: []string{fixture.tag("keep")}}
	if err := fixture.repository.UpdatePost(ctx, fixture.owner, serial, withTags, nil); err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}

	withoutTags := PostDraft{Title: "t2", Description: "d", AccessPolicy: PolicyPublic}
	if err := fixture.repository.UpdatePost(ctx, fixture.owner, serial, withoutTags, nil); err != nil {
		t.Fatalf("second UpdatePost() error = %v", err)
	}

	post, err := fixture.repository.Post(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if len(post.Tags) != 1 {
		t.Errorf("tags = %v, want the one that was there", post.Tags)
	}
}

// Two authors adding the same new tag at the same instant: the unique index on tags.name
// makes one of the inserts lose, and INSERT IGNORE is what keeps that from failing an
// author's whole edit.
func TestTwoPostsCanBeGivenTheSameNewTagAtOnce(t *testing.T) {
	fixture, ctx := newFixture(t)
	const posts = 6
	serials := make([]string, posts)
	for index := range serials {
		_, serials[index] = fixture.createPost(t, ctx, fixture.owner, fmt.Sprintf("post %d", index))
	}

	tag := fixture.tag("con")
	draft := PostDraft{Title: "t", Description: "d", AccessPolicy: PolicyPublic, Tags: []string{tag}}

	results := make(chan error, posts)
	start := make(chan struct{})
	for _, serial := range serials {
		go func(serial string) {
			<-start
			results <- fixture.repository.UpdatePost(ctx, fixture.owner, serial, draft, nil)
		}(serial)
	}
	close(start)
	for attempt := 0; attempt < posts; attempt++ {
		if err := <-results; err != nil {
			// Error 1213 is the deadlock this test exists for.
			t.Errorf("UpdatePost() error = %v", err)
		}
	}

	if fixture.count(t, ctx, `SELECT COUNT(*) FROM tags WHERE name = ?`, tag) != 1 {
		t.Error("the tag was created more than once")
	}
}

func TestPasswordHashReadsTheAccountsHash(t *testing.T) {
	fixture, ctx := newFixture(t)

	hash, err := fixture.repository.PasswordHash(ctx, fixture.owner)
	if err != nil {
		t.Fatalf("PasswordHash() error = %v", err)
	}
	// The fixture's users are created the way a Google sign-up creates them.
	if hash != "" {
		t.Errorf("hash = %q, want the empty string a passwordless account holds", hash)
	}
}

func TestWeekBoundariesAreMondays(t *testing.T) {
	cases := map[string]struct{ thisWeek, lastWeek string }{
		// A Thursday, a Monday and a Sunday, which is the one Go and Carbon disagree
		// about: Go counts Sunday as the start of the week and Carbon as the end.
		"2026-08-06": {"2026-08-03", "2026-07-27"},
		"2026-08-03": {"2026-08-03", "2026-07-27"},
		"2026-08-09": {"2026-08-03", "2026-07-27"},
	}
	for day, want := range cases {
		t.Run(day, func(t *testing.T) {
			parsed, err := time.Parse("2006-01-02", day)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			thisWeek, lastWeek := weekBoundaries(parsed)
			if thisWeek != want.thisWeek || lastWeek != want.lastWeek {
				t.Errorf("got %s and %s, want %s and %s", thisWeek, lastWeek, want.thisWeek, want.lastWeek)
			}
		})
	}
}

func TestPlayCountsComeFromThePostStatistics(t *testing.T) {
	fixture, ctx := newFixture(t)
	postID, serial := fixture.createPost(t, ctx, fixture.owner, "played")

	thisWeek, lastWeek := weekBoundaries(time.Now())
	for _, row := range []struct {
		timeRange string
		startDate string
		plays     int
	}{
		{"all", "1970-01-01", 500},
		{"week", thisWeek, 20},
		{"week", lastWeek, 30},
	} {
		if _, err := fixture.database.ExecContext(ctx,
			`INSERT INTO post_statistics (post_id, time_range, start_date, play_count, created_at, updated_at)
			 VALUES (?, ?, ?, ?, NOW(), NOW())`,
			postID, row.timeRange, row.startDate, row.plays); err != nil {
			t.Fatalf("seed statistics: %v", err)
		}
	}

	post, err := fixture.repository.Post(ctx, fixture.owner, serial)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if post.PlayCount != 500 {
		t.Errorf("play count = %d, want 500", post.PlayCount)
	}
	if post.ThisWeekPlayCount != 20 {
		t.Errorf("this week = %d, want 20", post.ThisWeekPlayCount)
	}
	if post.LastWeekPlayCount != 30 {
		t.Errorf("last week = %d, want 30", post.LastWeekPlayCount)
	}
}
