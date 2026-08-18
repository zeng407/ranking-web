package gameplay

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
	"2pick.app/backend/internal/postaccess"
)

/*
Access rules against the real database.

The unit tests prove the predicate is built correctly; only MySQL proves it does what the
text says once it is inside a five-table join. And the thing worth proving is negative — a
protected post that answers to nobody — which no amount of testing with public fixtures
would ever catch.

Every path is covered, not just Definition, because Laravel gated each of them separately:
a post that could be started but not resumed, or resumed but not submitted to, is a post
nobody can play.
*/

func accessTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_TEST_HOST")
	if host == "" {
		t.Skip("MYSQL_TEST_HOST is not set; skipping MySQL integration test")
	}
	port, err := strconv.Atoi(accessEnvOr("MYSQL_TEST_PORT", "3306"))
	if err != nil {
		t.Fatalf("MYSQL_TEST_PORT is not a number: %v", err)
	}
	database, err := mysqlstore.Open(config.DatabaseConfig{
		Host: host, Port: port,
		Name:            accessEnvOr("MYSQL_TEST_DATABASE", "rk_db_restore_20260729"),
		User:            accessEnvOr("MYSQL_TEST_USERNAME", "root"),
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

func accessEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type accessFixture struct {
	database   *sql.DB
	repository *MySQLRepository
	namespace  string
	owner      int64
	stranger   int64
	// serials are the fixture's posts by policy.
	serials map[string]string
}

func newAccessFixture(t *testing.T) (*accessFixture, context.Context) {
	t.Helper()
	database := accessTestDatabase(t)
	ctx := context.Background()

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("namespace: %v", err)
	}
	fixture := &accessFixture{
		database:   database,
		repository: NewMySQLRepository(database),
		namespace:  "go-access-" + hex.EncodeToString(suffix),
		serials:    map[string]string{},
	}

	// This fixture creates posts, which another package's test counts. See
	// sharedlock_test.go.
	lockSharedPosts(t, database)
	t.Cleanup(func() { fixture.cleanUp(t) })

	fixture.owner = fixture.createUser(t, ctx, "owner")
	fixture.stranger = fixture.createUser(t, ctx, "stranger")
	for _, policy := range []string{
		postaccess.PolicyPublic, postaccess.PolicyPrivate, postaccess.PolicyPassword,
	} {
		fixture.serials[policy] = fixture.createPost(t, ctx, policy)
	}
	return fixture, ctx
}

func (fixture *accessFixture) cleanUp(t *testing.T) {
	t.Helper()
	statements := []string{
		`DELETE ge FROM game_elements ge JOIN games g ON g.id = ge.game_id
		  JOIN posts p ON p.id = g.post_id JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE gr FROM game_1v1_rounds gr JOIN games g ON g.id = gr.game_id
		  JOIN posts p ON p.id = g.post_id JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE ugr FROM user_game_results ugr JOIN games g ON g.id = ugr.game_id
		  JOIN posts p ON p.id = g.post_id JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE g FROM games g JOIN posts p ON p.id = g.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE rr FROM rank_reports rr JOIN posts p ON p.id = rr.post_id
		  JOIN users u ON u.id = p.user_id WHERE u.email LIKE ?`,
		`DELETE pe FROM post_elements pe JOIN posts p ON p.id = pe.post_id
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
		`DELETE FROM elements WHERE source_url LIKE ?`, "https://"+fixture.namespace+"%"); err != nil {
		t.Errorf("clean up elements: %v", err)
	}
}

func (fixture *accessFixture) createUser(t *testing.T, ctx context.Context, name string) int64 {
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

// createPost writes one post under the given policy, with four elements — the smallest
// number that makes a two-round game.
func (fixture *accessFixture) createPost(t *testing.T, ctx context.Context, policy string) string {
	t.Helper()
	serial, err := newUUID()
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	serial = serial[:8]
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO posts (user_id, serial, title, description, created_at, updated_at)
		 VALUES (?, ?, ?, 'a description', NOW(), NOW())`,
		fixture.owner, serial, fixture.namespace+"-"+policy)
	if err != nil {
		t.Fatalf("create a post: %v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("post id: %v", err)
	}

	// The digest is what an author's editor writes and what PHP's
	// hash('sha256', $password) wrote before it.
	if _, err := fixture.database.ExecContext(ctx,
		`INSERT INTO post_policies (post_id, access_policy, password, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())`,
		postID, policy, postaccess.HashPassword("door-code")); err != nil {
		t.Fatalf("create a policy: %v", err)
	}

	for index := 0; index < 4; index++ {
		title := fmt.Sprintf("%s-%s-%d", fixture.namespace, policy, index)
		elementResult, err := fixture.database.ExecContext(ctx,
			`INSERT INTO elements (source_url, thumb_url, title, type, created_at, updated_at)
			 VALUES (?, ?, ?, 'image', NOW(), NOW())`,
			"https://"+title, "https://"+fixture.namespace+"/thumb", title)
		if err != nil {
			t.Fatalf("create an element: %v", err)
		}
		elementID, err := elementResult.LastInsertId()
		if err != nil {
			t.Fatalf("element id: %v", err)
		}
		if _, err := fixture.database.ExecContext(ctx,
			`INSERT INTO post_elements (post_id, element_id) VALUES (?, ?)`,
			postID, elementID); err != nil {
			t.Fatalf("attach the element: %v", err)
		}
	}
	return serial
}

// visitor is the zero caller: signed in as nobody, holding no door code.
var visitor = postaccess.Caller{}

func TestAPublicPostIsVisibleToAnyone(t *testing.T) {
	fixture, ctx := newAccessFixture(t)

	definition, err := fixture.repository.Definition(ctx, fixture.serials[postaccess.PolicyPublic], visitor)
	if err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
	if definition.ElementsCount != 4 {
		t.Errorf("elements = %d, want 4", definition.ElementsCount)
	}
}

func TestAProtectedPostDoesNotExistToAVisitor(t *testing.T) {
	fixture, ctx := newAccessFixture(t)

	for _, policy := range []string{postaccess.PolicyPrivate, postaccess.PolicyPassword} {
		if _, err := fixture.repository.Definition(ctx, fixture.serials[policy], visitor); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: Definition() error = %v, want ErrNotFound", policy, err)
		}
	}
}

func TestADoorCodeMakesAPasswordPostVisible(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	serial := fixture.serials[postaccess.PolicyPassword]
	unlocked := postaccess.Caller{UnlockedSerials: []string{serial}}

	if _, err := fixture.repository.Definition(ctx, serial, unlocked); err != nil {
		t.Fatalf("Definition() error = %v", err)
	}
}

/*
A DOOR CODE OPENS ONE DOOR.

Holding post A's token must not make post B visible, even when both belong to the same
author and both use a password. Without the serial in the predicate's IN list this passes
anyway, which is why the private post is checked in the same breath: a token must not open
a post that has no password at all.
*/
func TestADoorCodeDoesNotOpenAnotherPost(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	unlocked := postaccess.Caller{UnlockedSerials: []string{fixture.serials[postaccess.PolicyPassword]}}

	if _, err := fixture.repository.Definition(
		ctx, fixture.serials[postaccess.PolicyPrivate], unlocked); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Definition() error = %v, want ErrNotFound", err)
	}
}

// A token for a post that has since been switched to private must stop working. The
// predicate checks the policy as well as the serial, so the answer follows the column
// rather than the token.
func TestATokenDoesNotSurviveThePolicyChanging(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	serial := fixture.serials[postaccess.PolicyPassword]
	unlocked := postaccess.Caller{UnlockedSerials: []string{serial}}

	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE post_policies pp JOIN posts p ON p.id = pp.post_id
		    SET pp.access_policy = 'private' WHERE p.serial = ?`, serial); err != nil {
		t.Fatalf("switch the policy: %v", err)
	}

	if _, err := fixture.repository.Definition(ctx, serial, unlocked); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Definition() error = %v, want ErrNotFound", err)
	}
}

// The author reads their own posts without a door code, which is what GamePolicy::play
// did before it looked at the token at all.
func TestTheAuthorNeedsNoDoorCode(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	author := postaccess.Caller{UserID: fixture.owner}

	for _, policy := range []string{postaccess.PolicyPrivate, postaccess.PolicyPassword} {
		if _, err := fixture.repository.Definition(ctx, fixture.serials[policy], author); err != nil {
			t.Errorf("%s: Definition() error = %v", policy, err)
		}
	}
}

func TestAnotherAccountIsNotTheAuthor(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	stranger := postaccess.Caller{UserID: fixture.stranger}

	for _, policy := range []string{postaccess.PolicyPrivate, postaccess.PolicyPassword} {
		if _, err := fixture.repository.Definition(ctx, fixture.serials[policy], stranger); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: Definition() error = %v, want ErrNotFound", policy, err)
		}
	}
}

/*
A PROTECTED POST MUST BE PLAYABLE ALL THE WAY THROUGH, AND ONLY BY WHO MAY.

Laravel gated creation, resume, submission and the result separately. Widening only
Create would produce a post that starts a game and then refuses to continue it — worse
than not opening at all, because the player has already spent the votes.
*/
func TestAPasswordPostPlaysThroughForAHolderAndForNobodyElse(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	serial := fixture.serials[postaccess.PolicyPassword]
	unlocked := postaccess.Caller{UnlockedSerials: []string{serial}}

	session, err := fixture.repository.Create(ctx, CreateInput{
		PostSerial: serial, ElementCount: 4, Caller: unlocked,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := fixture.repository.Resume(ctx, session.GameSerial, unlocked); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if _, err := fixture.repository.Resume(ctx, session.GameSerial, visitor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resume() by a visitor error = %v, want ErrNotFound", err)
	}

	// Two semi-finals then a final, which is the whole of a four-element bracket.
	votes := []Vote{
		{WinnerID: session.Elements[0].ID, LoserID: session.Elements[1].ID},
		{WinnerID: session.Elements[2].ID, LoserID: session.Elements[3].ID},
	}
	if _, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 0, Votes: votes, Caller: visitor,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SubmitVotes() by a visitor error = %v, want ErrNotFound", err)
	}
	if _, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 0, Votes: votes, Caller: unlocked,
	}); err != nil {
		t.Fatalf("SubmitVotes() error = %v", err)
	}
	final, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 2,
		Votes:             []Vote{{WinnerID: session.Elements[0].ID, LoserID: session.Elements[2].ID}},
		Caller:            unlocked,
	})
	if err != nil {
		t.Fatalf("SubmitVotes() final error = %v", err)
	}
	if !final.Complete {
		t.Fatalf("the game did not complete: %#v", final)
	}

	if _, err := fixture.repository.Result(ctx, session.GameSerial, unlocked); err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if _, err := fixture.repository.Result(ctx, session.GameSerial, visitor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Result() by a visitor error = %v, want ErrNotFound", err)
	}
}

func TestAVisitorCannotStartAGameOnAProtectedPost(t *testing.T) {
	fixture, ctx := newAccessFixture(t)

	for _, policy := range []string{postaccess.PolicyPrivate, postaccess.PolicyPassword} {
		_, err := fixture.repository.Create(ctx, CreateInput{
			PostSerial: fixture.serials[policy], ElementCount: 4, Caller: visitor,
		})
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: Create() error = %v, want ErrNotFound", policy, err)
		}
	}
}

// markCensored sets posts.is_censored, which no fixture post carries by default.
func (fixture *accessFixture) markCensored(t *testing.T, ctx context.Context, serial string) {
	t.Helper()
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE posts SET is_censored = 1 WHERE serial = ?`, serial); err != nil {
		t.Fatalf("mark the post censored: %v", err)
	}
}

/*
AN ADULT POST PREVIEWS FOR ANYONE AND PLAYS ONLY FOR AN ACCOUNT.

Definition stays open on purpose: the post is listed on the home page and its two preview
thumbnails are shown on the game page behind a blur, so a visitor has to be able to read it.
Everything past the preview needs an account, and — as with a door code — every path needs
it, or a visitor starts a game that then refuses to continue after the votes are spent.

The refusal is postaccess.ErrSignInRequired rather than ErrNotFound because the post is
public: hiding it would contradict the listing the visitor just clicked, and the browser
could not tell "sign in" apart from "enter the door code".
*/
func TestAnAdultPostPreviewsForAnyoneAndPlaysOnlyForAnAccount(t *testing.T) {
	fixture, ctx := newAccessFixture(t)
	serial := fixture.serials[postaccess.PolicyPublic]
	fixture.markCensored(t, ctx, serial)
	account := postaccess.Caller{UserID: fixture.stranger}

	definition, err := fixture.repository.Definition(ctx, serial, visitor)
	if err != nil {
		t.Fatalf("Definition() by a visitor error = %v, want the preview to work", err)
	}
	if !definition.IsCensored {
		t.Fatalf("Definition() reported is_censored = false for a censored post")
	}

	if _, err := fixture.repository.Create(ctx, CreateInput{
		PostSerial: serial, ElementCount: 4, Caller: visitor,
	}); !errors.Is(err, postaccess.ErrSignInRequired) {
		t.Fatalf("Create() by a visitor error = %v, want ErrSignInRequired", err)
	}

	session, err := fixture.repository.Create(ctx, CreateInput{
		PostSerial: serial, ElementCount: 4, Caller: account,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := fixture.repository.Resume(ctx, session.GameSerial, visitor); !errors.Is(err, postaccess.ErrSignInRequired) {
		t.Fatalf("Resume() by a visitor error = %v, want ErrSignInRequired", err)
	}
	if _, err := fixture.repository.Resume(ctx, session.GameSerial, account); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	votes := []Vote{
		{WinnerID: session.Elements[0].ID, LoserID: session.Elements[1].ID},
		{WinnerID: session.Elements[2].ID, LoserID: session.Elements[3].ID},
	}
	if _, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 0, Votes: votes, Caller: visitor,
	}); !errors.Is(err, postaccess.ErrSignInRequired) {
		t.Fatalf("SubmitVotes() by a visitor error = %v, want ErrSignInRequired", err)
	}
	if _, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 0, Votes: votes, Caller: account,
	}); err != nil {
		t.Fatalf("SubmitVotes() error = %v", err)
	}
	final, err := fixture.repository.SubmitVotes(ctx, session.GameSerial, BatchInput{
		ExpectedVoteCount: 2,
		Votes:             []Vote{{WinnerID: session.Elements[0].ID, LoserID: session.Elements[2].ID}},
		Caller:            account,
	})
	if err != nil {
		t.Fatalf("SubmitVotes() final error = %v", err)
	}
	if !final.Complete {
		t.Fatalf("the game did not complete: %#v", final)
	}

	if _, err := fixture.repository.Result(ctx, session.GameSerial, visitor); !errors.Is(err, postaccess.ErrSignInRequired) {
		t.Fatalf("Result() by a visitor error = %v, want ErrSignInRequired", err)
	}
	if _, err := fixture.repository.Result(ctx, session.GameSerial, account); err != nil {
		t.Fatalf("Result() error = %v", err)
	}
}
