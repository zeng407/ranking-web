package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// These run against the real table because the parts that matter cannot be faked:
// whether the unique index added in 00009 actually rejects a second link, whether
// isDuplicateKey recognises what the driver returns when it does, and whether the
// two-row account creation is really atomic.

type socialFixture struct {
	store    *MySQLSocialStore
	database *sql.DB
	// namespace makes every row this test writes findable, so the cleanup can be
	// exact rather than a guess.
	namespace string
}

func newSocialFixture(t *testing.T) (*socialFixture, context.Context) {
	t.Helper()
	database := testDatabase(t)

	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate a namespace: %v", err)
	}
	fixture := &socialFixture{
		store:     NewMySQLSocialStore(database),
		database:  database,
		namespace: fmt.Sprintf("gotest-%x", raw),
	}

	t.Cleanup(func() {
		// The socialite rows go with the users: the foreign key is ON DELETE CASCADE.
		// Anything attached to a pre-existing account is removed first, since that row
		// has no user of ours to cascade from.
		if _, err := database.ExecContext(context.Background(),
			`DELETE FROM user_socialities WHERE google_id LIKE ?`, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up test links: %v", err)
		}
		if _, err := database.ExecContext(context.Background(),
			`DELETE FROM users WHERE email LIKE ?`, fixture.namespace+"%"); err != nil {
			t.Errorf("clean up test users: %v", err)
		}
	})
	return fixture, context.Background()
}

func (fixture *socialFixture) email(name string) string {
	return fmt.Sprintf("%s-%s@invalid.test", fixture.namespace, name)
}

func (fixture *socialFixture) subject(name string) string {
	return fmt.Sprintf("%s-%s", fixture.namespace, name)
}

// createBareUser makes an account with no provider link, for the Link tests.
func (fixture *socialFixture) createBareUser(t *testing.T, ctx context.Context, name string) int64 {
	t.Helper()
	result, err := fixture.database.ExecContext(ctx,
		`INSERT INTO users (name, email, password, created_at, updated_at) VALUES (?, ?, '', NOW(), NOW())`,
		"go test", fixture.email(name))
	if err != nil {
		t.Fatalf("create a bare user: %v", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("bare user id: %v", err)
	}
	return userID
}

func (fixture *socialFixture) newLinkedUser(name string) NewLinkedUser {
	return NewLinkedUser{
		Provider:      ProviderGoogle,
		Subject:       fixture.subject(name),
		Email:         fixture.email(name),
		Name:          "go test",
		AvatarURL:     "https://example.test/avatar.png",
		EmailVerified: true,
		CreatedAt:     time.Now().UTC(),
	}
}

func TestCreateLinkedUserWritesBothRows(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	record := fixture.newLinkedUser("new-account")

	credentials, err := fixture.store.CreateLinkedUser(ctx, record)
	if err != nil {
		t.Fatalf("CreateLinkedUser() error = %v", err)
	}
	if credentials.UserID == 0 {
		t.Fatal("no user id was returned")
	}
	// Empty rather than nil, so the roles claim encodes as [] and not null.
	if credentials.Roles == nil {
		t.Error("roles is nil")
	}

	// THE PASSWORD MUST BE THE EMPTY STRING, NOT NULL. The column is NOT NULL, and the
	// login path's guard is written against "" — a NULL here would fail the scan in
	// FindByEmail and lock the account out of every future password login attempt in a
	// way that looks like a database error.
	var (
		password      string
		verifiedAt    sql.NullTime
		avatar        sql.NullString
		rememberToken sql.NullString
	)
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT password, email_verified_at, avatar_url, remember_token FROM users WHERE id = ?`,
		credentials.UserID).Scan(&password, &verifiedAt, &avatar, &rememberToken); err != nil {
		t.Fatalf("read the new account: %v", err)
	}
	if password != "" {
		t.Errorf("password = %q, want the empty string", password)
	}
	if !verifiedAt.Valid {
		t.Error("email_verified_at is NULL although the provider verified the address")
	}
	if avatar.String != record.AvatarURL {
		t.Errorf("avatar_url = %q", avatar.String)
	}
	if rememberToken.Valid {
		t.Error("remember_token was written; it belongs to Laravel's cookie session")
	}

	// And the link is reachable by subject, which is the only way a returning user is
	// found.
	found, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, record.Subject)
	if err != nil {
		t.Fatalf("the link was not written: %v", err)
	}
	if found.UserID != credentials.UserID {
		t.Errorf("the link points at %d, want %d", found.UserID, credentials.UserID)
	}
}

// An unverified address must not produce a verified timestamp. Laravel wrote NULL in
// that case and so must this.
func TestCreateLinkedUserLeavesAnUnverifiedAddressUnverified(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	record := fixture.newLinkedUser("unverified")
	record.EmailVerified = false

	credentials, err := fixture.store.CreateLinkedUser(ctx, record)
	if err != nil {
		t.Fatalf("CreateLinkedUser() error = %v", err)
	}

	var verifiedAt sql.NullTime
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT email_verified_at FROM users WHERE id = ?`, credentials.UserID).Scan(&verifiedAt); err != nil {
		t.Fatalf("read the new account: %v", err)
	}
	if verifiedAt.Valid {
		t.Errorf("email_verified_at = %v, want NULL", verifiedAt.Time)
	}
}

// THE UNIQUE INDEX FROM 00009, AND isDuplicateKey RECOGNISING IT. This is the pair that
// cannot be tested against a fake: the fake returns the error by construction, while
// here the driver has to produce error 1062 and the store has to classify it.
func TestASecondAccountCannotClaimTheSameProviderSubject(t *testing.T) {
	fixture, ctx := newSocialFixture(t)

	first := fixture.newLinkedUser("first")
	if _, err := fixture.store.CreateLinkedUser(ctx, first); err != nil {
		t.Fatalf("first account: %v", err)
	}

	// Same subject, different address: this is the concurrent-signup case.
	second := fixture.newLinkedUser("second")
	second.Subject = first.Subject

	_, err := fixture.store.CreateLinkedUser(ctx, second)
	if !errors.Is(err, ErrOAuthAlreadyLinked) {
		t.Fatalf("error = %v, want ErrOAuthAlreadyLinked", err)
	}

	// AND THE FIRST STATEMENT MUST HAVE ROLLED BACK. The user row was inserted before
	// the link failed; if the transaction leaked, that address is now taken by an
	// account with no password and no provider link — one nothing can ever sign into,
	// and whose presence makes the retry fail with "email taken" forever.
	var orphans int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE email = ?`, second.Email).Scan(&orphans); err != nil {
		t.Fatalf("look for an orphaned account: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d unreachable accounts were left behind by the failed link", orphans)
	}
}

// users.email is UNIQUE, so the address check is backed by the database rather than only
// by the read that precedes it.
func TestCreateLinkedUserReportsATakenAddress(t *testing.T) {
	fixture, ctx := newSocialFixture(t)

	first := fixture.newLinkedUser("taken")
	if _, err := fixture.store.CreateLinkedUser(ctx, first); err != nil {
		t.Fatalf("first account: %v", err)
	}

	// Same address, different subject.
	second := fixture.newLinkedUser("taken")
	second.Subject = fixture.subject("someone-else")

	_, err := fixture.store.CreateLinkedUser(ctx, second)
	if !errors.Is(err, ErrOAuthEmailTaken) {
		t.Fatalf("error = %v, want ErrOAuthEmailTaken", err)
	}
}

func TestEmailExistsIsCaseInsensitive(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	record := fixture.newLinkedUser("case")
	if _, err := fixture.store.CreateLinkedUser(ctx, record); err != nil {
		t.Fatalf("create the account: %v", err)
	}

	for _, variant := range []string{record.Email, strings.ToUpper(record.Email)} {
		exists, err := fixture.store.EmailExists(ctx, variant)
		if err != nil {
			t.Fatalf("EmailExists(%q): %v", variant, err)
		}
		if !exists {
			t.Errorf("EmailExists(%q) = false; the collation is not case-insensitive", variant)
		}
	}

	exists, err := fixture.store.EmailExists(ctx, fixture.email("never-registered"))
	if err != nil {
		t.Fatalf("EmailExists on an unknown address: %v", err)
	}
	if exists {
		t.Error("an unregistered address reported as taken")
	}
}

func TestFindByProviderSubjectMissesRatherThanErrors(t *testing.T) {
	fixture, ctx := newSocialFixture(t)

	for name, subject := range map[string]string{
		"unknown": fixture.subject("never-linked"),
		// An empty subject must not match the rows where google_id is NULL.
		"empty": "",
	} {
		_, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, subject)
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("%s subject: error = %v, want ErrUserNotFound", name, err)
		}
	}
}

func TestAnUnknownProviderIsRejectedEverywhere(t *testing.T) {
	fixture, ctx := newSocialFixture(t)

	if _, err := fixture.store.FindByProviderSubject(ctx, "twitch", "x"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("FindByProviderSubject: error = %v, want ErrUnknownProvider", err)
	}
	record := fixture.newLinkedUser("wrong-provider")
	record.Provider = "twitch"
	if _, err := fixture.store.CreateLinkedUser(ctx, record); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("CreateLinkedUser: error = %v, want ErrUnknownProvider", err)
	}
	if err := fixture.store.Link(ctx, LinkRequest{
		UserID: 1, Provider: "twitch", Subject: "x",
	}); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("Link: error = %v, want ErrUnknownProvider", err)
	}
}

func TestLinkAttachesAProviderToAnExistingAccount(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	userID := fixture.createBareUser(t, ctx, "linker")
	subject := fixture.subject("linked")

	if err := fixture.store.Link(ctx, LinkRequest{
		UserID: userID, Provider: ProviderGoogle, Subject: subject, Email: fixture.email("linker"),
	}); err != nil {
		t.Fatalf("Link() error = %v", err)
	}

	found, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, subject)
	if err != nil {
		t.Fatalf("the link is not resolvable: %v", err)
	}
	if found.UserID != userID {
		t.Errorf("the link points at %d, want %d", found.UserID, userID)
	}
}

// A user who already has a Google account must not have it silently replaced: that would
// move the login of whoever owns the old one.
func TestLinkRefusesToReplaceAnExistingLink(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	userID := fixture.createBareUser(t, ctx, "already-linked")

	if err := fixture.store.Link(ctx, LinkRequest{
		UserID: userID, Provider: ProviderGoogle, Subject: fixture.subject("first"),
	}); err != nil {
		t.Fatalf("first link: %v", err)
	}

	err := fixture.store.Link(ctx, LinkRequest{
		UserID: userID, Provider: ProviderGoogle, Subject: fixture.subject("second"),
	})
	if !errors.Is(err, ErrOAuthAlreadyLinked) {
		t.Fatalf("error = %v, want ErrOAuthAlreadyLinked", err)
	}

	// The original link survived.
	found, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, fixture.subject("first"))
	if err != nil || found.UserID != userID {
		t.Errorf("the original link was disturbed: %v, %+v", err, found)
	}
}

// The same provider account must not end up on two users.
func TestLinkRefusesAProviderAccountHeldByAnotherUser(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	first := fixture.createBareUser(t, ctx, "holder")
	second := fixture.createBareUser(t, ctx, "thief")
	subject := fixture.subject("contested")

	if err := fixture.store.Link(ctx, LinkRequest{
		UserID: first, Provider: ProviderGoogle, Subject: subject,
	}); err != nil {
		t.Fatalf("first link: %v", err)
	}

	err := fixture.store.Link(ctx, LinkRequest{
		UserID: second, Provider: ProviderGoogle, Subject: subject,
	})
	if !errors.Is(err, ErrOAuthAlreadyLinked) {
		t.Fatalf("error = %v, want ErrOAuthAlreadyLinked; the unique index should have stopped this", err)
	}

	found, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, subject)
	if err != nil {
		t.Fatalf("the link disappeared: %v", err)
	}
	if found.UserID != first {
		t.Errorf("the provider account moved to user %d, want it to stay on %d", found.UserID, first)
	}
}

// Concurrent links for the same provider account: exactly one may win, and the losers
// must say so rather than surfacing a raw driver error.
func TestConcurrentLinksOfOneProviderAccountAllowExactlyOne(t *testing.T) {
	fixture, ctx := newSocialFixture(t)
	subject := fixture.subject("race")

	const racers = 4
	users := make([]int64, racers)
	for index := range users {
		users[index] = fixture.createBareUser(t, ctx, fmt.Sprintf("racer-%d", index))
	}

	type outcome struct{ err error }
	results := make(chan outcome, racers)
	for _, userID := range users {
		go func(userID int64) {
			results <- outcome{err: fixture.store.Link(ctx, LinkRequest{
				UserID: userID, Provider: ProviderGoogle, Subject: subject,
			})}
		}(userID)
	}

	var winners int
	var unexpected []error
	for range racers {
		result := <-results
		switch {
		case result.err == nil:
			winners++
		case errors.Is(result.err, ErrOAuthAlreadyLinked):
		default:
			unexpected = append(unexpected, result.err)
		}
	}

	if winners != 1 {
		t.Errorf("%d of %d concurrent links succeeded, want exactly 1", winners, racers)
	}
	if len(unexpected) > 0 {
		t.Errorf("a losing link surfaced a raw error instead of ErrOAuthAlreadyLinked: %v", unexpected)
	}
}

// An account reached through OAuth must get the same roles it gets through a password.
//
// THIS IS NOT HYPOTHETICAL. There is exactly one role holder in production — the single
// admin — and that account has a Google link, so administration depends on this lookup
// returning the role. If FindByProviderSubject ever stopped reading roles, the symptom
// would be an admin who signs in successfully and then finds every admin page refused.
//
// Read-only: it asserts against whoever actually holds a role rather than creating one.
func TestALinkedAccountReadsTheSameRolesAsAPasswordAccount(t *testing.T) {
	fixture, ctx := newSocialFixture(t)

	var (
		userID  int64
		subject string
	)
	err := fixture.database.QueryRowContext(ctx, `
		SELECT ur.user_id, s.google_id
		  FROM user_roles AS ur
		  JOIN user_socialities AS s ON s.user_id = ur.user_id
		 WHERE s.google_id IS NOT NULL
		 ORDER BY ur.user_id
		 LIMIT 1`).Scan(&userID, &subject)
	if errors.Is(err, sql.ErrNoRows) {
		t.Skip("no role holder has a provider link in this database")
	}
	if err != nil {
		t.Fatalf("find a role holder with a link: %v", err)
	}

	viaProvider, err := fixture.store.FindByProviderSubject(ctx, ProviderGoogle, subject)
	if err != nil {
		t.Fatalf("look up by subject: %v", err)
	}
	viaID, err := NewMySQLUserStore(fixture.database).FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("look up by id: %v", err)
	}

	if viaProvider.UserID != userID {
		t.Errorf("the provider lookup resolved to %d, want %d", viaProvider.UserID, userID)
	}
	if len(viaProvider.Roles) == 0 {
		t.Fatal("the provider lookup returned no roles for a user who has one; every admin page would refuse them")
	}
	if strings.Join(viaProvider.Roles, ",") != strings.Join(viaID.Roles, ",") {
		t.Errorf("roles differ by path: %v via provider, %v via id", viaProvider.Roles, viaID.Roles)
	}
}
