package postaccess

import (
	"context"
	"errors"
	"strings"
	"testing"
)

/*
The service. What matters is that a door code opens exactly the post it belongs to, that
guessing costs something, and that refusals do not tell a stranger which posts are worth
attacking.
*/

type fakeStore struct {
	posts map[string]Post
	err   error
}

func (store fakeStore) Post(_ context.Context, serial string) (Post, error) {
	if store.err != nil {
		return Post{}, store.err
	}
	post, found := store.posts[serial]
	if !found {
		return Post{}, ErrPostNotFound
	}
	return post, nil
}

type countingAttempts struct {
	used  int
	limit int
	err   error
}

func (attempts *countingAttempts) Allow(context.Context, string) (bool, error) {
	if attempts.err != nil {
		return false, attempts.err
	}
	attempts.used++
	return attempts.used <= attempts.limit, nil
}

func lockedPost(serial, password string) Post {
	return Post{ID: 1, Serial: serial, OwnerID: 7, Policy: PolicyPassword,
		PasswordDigest: HashPassword(password)}
}

func newService(t *testing.T, store Store, attempts Attempts) *Service {
	t.Helper()
	service, err := NewService(ServiceOptions{Store: store, Signer: newSigner(t), Attempts: attempts})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestTheRightPasswordIssuesAToken(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"abcdefgh": lockedPost("abcdefgh", "door-code")}}
	service := newService(t, store, nil)

	token, expiresAt, err := service.Grant(context.Background(), "abcdefgh", "door-code")
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	if err := service.Verify("abcdefgh", token); err != nil {
		t.Errorf("the issued token does not verify: %v", err)
	}
	if expiresAt.IsZero() {
		t.Error("no expiry was reported")
	}
}

func TestTheWrongPasswordIssuesNothing(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"abcdefgh": lockedPost("abcdefgh", "door-code")}}
	service := newService(t, store, nil)

	_, _, err := service.Grant(context.Background(), "abcdefgh", "not-the-code")

	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("error = %v, want ErrWrongPassword", err)
	}
}

/*
A POST WITH NO PASSWORD ANSWERS THE SAME AS A WRONG PASSWORD.

Answering "this post has no password" would let anyone enumerate which posts are protected
— and a public post's rank is readable without any of this, so there is nothing to gain by
being helpful here.
*/
func TestAPostWithNoPasswordCannotBeUnlocked(t *testing.T) {
	store := fakeStore{posts: map[string]Post{
		"public00": {ID: 2, Serial: "public00", Policy: PolicyPublic},
		"private0": {ID: 3, Serial: "private0", Policy: PolicyPrivate},
	}}
	service := newService(t, store, nil)

	for _, serial := range []string{"public00", "private0"} {
		if _, _, err := service.Grant(context.Background(), serial, ""); !errors.Is(err, ErrWrongPassword) {
			t.Errorf("%s: error = %v, want ErrWrongPassword", serial, err)
		}
	}
}

func TestAnUnknownSerialIsNotFound(t *testing.T) {
	service := newService(t, fakeStore{posts: map[string]Post{}}, nil)

	_, _, err := service.Grant(context.Background(), "nosuchpo", "door-code")

	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("error = %v, want ErrPostNotFound", err)
	}
}

// Ten a minute per post, as GameController::access allowed. The eleventh is refused
// whether or not it was right.
func TestGuessingIsRateLimited(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"abcdefgh": lockedPost("abcdefgh", "door-code")}}
	attempts := &countingAttempts{limit: RateLimit}
	service := newService(t, store, attempts)

	for guess := 0; guess < RateLimit; guess++ {
		if _, _, err := service.Grant(context.Background(), "abcdefgh", "wrong"); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("guess %d: error = %v, want ErrWrongPassword", guess, err)
		}
	}

	_, _, err := service.Grant(context.Background(), "abcdefgh", "door-code")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited — the correct password was let through past the limit", err)
	}
}

// The budget is charged before the comparison, so a right guess costs the same as a wrong
// one. Charging only failures would let an attacker who gets close probe for free.
func TestASuccessfulUnlockSpendsBudgetToo(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"abcdefgh": lockedPost("abcdefgh", "door-code")}}
	attempts := &countingAttempts{limit: RateLimit}
	service := newService(t, store, attempts)

	if _, _, err := service.Grant(context.Background(), "abcdefgh", "door-code"); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	if attempts.used != 1 {
		t.Errorf("attempts charged = %d, want 1", attempts.used)
	}
}

// A post that is not password-protected must not spend anyone's budget: otherwise anyone
// could exhaust a post's limiter by hammering a serial that has no password at all.
func TestAPostWithNoPasswordSpendsNoBudget(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"public00": {Serial: "public00", Policy: PolicyPublic}}}
	attempts := &countingAttempts{limit: RateLimit}
	service := newService(t, store, attempts)

	_, _, _ = service.Grant(context.Background(), "public00", "anything")

	if attempts.used != 0 {
		t.Errorf("attempts charged = %d, want 0", attempts.used)
	}
}

func TestARateLimiterFailureRefusesRatherThanLetsThrough(t *testing.T) {
	store := fakeStore{posts: map[string]Post{"abcdefgh": lockedPost("abcdefgh", "door-code")}}
	broken := &countingAttempts{err: errors.New("redis is down")}
	service := newService(t, store, broken)

	if _, _, err := service.Grant(context.Background(), "abcdefgh", "door-code"); err == nil {
		t.Fatal("a broken rate limiter let the request through")
	}
}

func TestCallerForKeepsOnlyTokensThatVerify(t *testing.T) {
	service := newService(t, fakeStore{}, nil)
	good, _ := service.Reissue("abcdefgh")

	caller := service.CallerFor(42, map[string]string{
		"abcdefgh": good,
		"ijklmnop": "rubbish",
		"qrstuvwx": good, // the right shape, but minted for a different post
	})

	if caller.UserID != 42 {
		t.Errorf("UserID = %d, want 42", caller.UserID)
	}
	if len(caller.UnlockedSerials) != 1 || caller.UnlockedSerials[0] != "abcdefgh" {
		t.Fatalf("unlocked = %v, want [abcdefgh]", caller.UnlockedSerials)
	}
	if !caller.Unlocked("abcdefgh") || caller.Unlocked("qrstuvwx") {
		t.Error("Unlocked() disagrees with the list it was built from")
	}
}

func TestNewServiceRequiresItsParts(t *testing.T) {
	if _, err := NewService(ServiceOptions{Signer: newSigner(t)}); err == nil {
		t.Error("NewService() accepted no store")
	}
	if _, err := NewService(ServiceOptions{Store: fakeStore{}}); err == nil {
		t.Error("NewService() accepted no signer")
	}
}

/*
The visibility predicate. These tests read the SQL as text, which is unusual, but the
clause is the whole of the access rule — a missing branch here is a post everyone can
read, and it would not show up as a failure anywhere that only exercises public posts.
*/

func TestAVisitorSeesOnlyPublicPosts(t *testing.T) {
	clause, arguments := VisibilityClause("p", "pp", Caller{})

	if clause != "(pp.access_policy = 'public')" {
		t.Errorf("clause = %q", clause)
	}
	if len(arguments) != 0 {
		t.Errorf("arguments = %v, want none", arguments)
	}
}

func TestASignedInCallerAlsoSeesTheirOwnPosts(t *testing.T) {
	clause, arguments := VisibilityClause("p", "pp", Caller{UserID: 42})

	if !contains(clause, "p.user_id = ?") {
		t.Errorf("clause = %q, want the owner branch", clause)
	}
	if len(arguments) != 1 || arguments[0] != int64(42) {
		t.Errorf("arguments = %v, want [42]", arguments)
	}
}

// User 0 is "nobody signed in". Binding it would match every post whose user_id is NULL —
// COALESCEd to 0 elsewhere — and hand a visitor the orphaned posts.
func TestUserZeroDoesNotProduceAnOwnerBranch(t *testing.T) {
	clause, _ := VisibilityClause("p", "pp", Caller{UserID: 0})

	if contains(clause, "user_id") {
		t.Errorf("clause = %q, want no owner branch for an anonymous caller", clause)
	}
}

// An unlocked serial widens the predicate only for password posts. Without that guard, a
// stale token for a post whose author has since switched it to private would still open
// it.
func TestAnUnlockedSerialOnlyOpensAPasswordPost(t *testing.T) {
	clause, arguments := VisibilityClause("p", "pp",
		Caller{UnlockedSerials: []string{"abcdefgh", "ijklmnop"}})

	if !contains(clause, "pp.access_policy = 'password' AND p.serial IN (?, ?)") {
		t.Errorf("clause = %q", clause)
	}
	if len(arguments) != 2 || arguments[0] != "abcdefgh" || arguments[1] != "ijklmnop" {
		t.Errorf("arguments = %v, want the two serials", arguments)
	}
}

// The arguments must come out in the order the placeholders appear, or the query binds the
// user id to a serial and silently matches nothing.
func TestArgumentsFollowThePlaceholderOrder(t *testing.T) {
	clause, arguments := VisibilityClause("p", "pp",
		Caller{UserID: 42, UnlockedSerials: []string{"abcdefgh"}})

	ownerAt := index(clause, "p.user_id = ?")
	serialAt := index(clause, "p.serial IN")
	if ownerAt < 0 || serialAt < 0 || ownerAt > serialAt {
		t.Fatalf("clause = %q, want the owner branch before the serial branch", clause)
	}
	if len(arguments) != 2 || arguments[0] != int64(42) || arguments[1] != "abcdefgh" {
		t.Errorf("arguments = %v, want [42 abcdefgh]", arguments)
	}
}

func TestTheAliasesAreTheOnesTheCallerAsksFor(t *testing.T) {
	clause, _ := VisibilityClause("post", "policy", Caller{UserID: 1, UnlockedSerials: []string{"a"}})

	for _, want := range []string{"policy.access_policy", "post.user_id", "post.serial"} {
		if !contains(clause, want) {
			t.Errorf("clause = %q, want %s", clause, want)
		}
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func index(haystack, needle string) int { return strings.Index(haystack, needle) }

/*
THE ADULT RULE IS ABOUT ACCOUNTS, NOT ABOUT DOOR CODES.

An 18+ post is listed on the home page and previewed on the game page, so the rule cannot
live in VisibilityClause — it only applies to playing, voting and reading the ranking. A
door code is no substitute for an account here: the two protections answer different
questions, and a password post that is also 18+ needs both.
*/
func TestAnAdultPostNeedsAnAccount(t *testing.T) {
	for name, testCase := range map[string]struct {
		isCensored bool
		caller     Caller
		wantErr    error
	}{
		"a visitor may read an ordinary post":         {false, Caller{}, nil},
		"a visitor may not read an adult post":        {true, Caller{}, ErrSignInRequired},
		"an account may read an adult post":           {true, Caller{UserID: 7}, nil},
		"a door code is not an account":               {true, Caller{UnlockedSerials: []string{"abc"}}, ErrSignInRequired},
		"an account with a door code reads it anyway": {true, Caller{UserID: 7, UnlockedSerials: []string{"abc"}}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			if err := RequireSignIn(testCase.isCensored, testCase.caller); !errors.Is(err, testCase.wantErr) {
				t.Errorf("RequireSignIn() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}
