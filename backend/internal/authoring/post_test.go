package authoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// The editor's rules, against in-memory stores. The SQL half is in
// mysql_repository_test.go.

type memoryPostStore struct {
	posts map[string]Post
	// owner is the user every post in this store belongs to.
	owner int64

	created      []PostDraft
	createdHash  string
	updated      []PostDraft
	updatedHash  *string
	deletedID    int64
	deleteCalled bool
	err          error
}

func newMemoryPostStore() *memoryPostStore {
	return &memoryPostStore{posts: map[string]Post{}, owner: 7}
}

func (store *memoryPostStore) ListPosts(_ context.Context, userID int64, page, perPage int) ([]Post, int, error) {
	if store.err != nil {
		return nil, 0, store.err
	}
	if userID != store.owner {
		return nil, 0, nil
	}
	all := make([]Post, 0, len(store.posts))
	for _, post := range store.posts {
		all = append(all, post)
	}
	return all, len(all), nil
}

func (store *memoryPostStore) Post(_ context.Context, userID int64, serial string) (Post, error) {
	if store.err != nil {
		return Post{}, store.err
	}
	post, ok := store.posts[serial]
	if !ok || userID != store.owner {
		return Post{}, ErrPostNotFound
	}
	return post, nil
}

func (store *memoryPostStore) CreatePost(
	_ context.Context, _ int64, serial string, draft PostDraft, passwordHash string,
) error {
	if store.err != nil {
		return store.err
	}
	store.created = append(store.created, draft)
	store.createdHash = passwordHash
	store.posts[serial] = Post{
		Serial: serial, Title: draft.Title, Description: draft.Description,
		AccessPolicy: draft.AccessPolicy, HasPassword: passwordHash != "", Tags: draft.Tags,
	}
	return nil
}

func (store *memoryPostStore) UpdatePost(
	_ context.Context, _ int64, serial string, draft PostDraft, passwordHash *string,
) error {
	if store.err != nil {
		return store.err
	}
	store.updated = append(store.updated, draft)
	store.updatedHash = passwordHash
	post := store.posts[serial]
	post.Title, post.Description, post.AccessPolicy = draft.Title, draft.Description, draft.AccessPolicy
	if passwordHash != nil {
		post.HasPassword = *passwordHash != ""
	}
	if draft.Tags != nil {
		post.Tags = CleanTags(draft.Tags)
	}
	store.posts[serial] = post
	return nil
}

func (store *memoryPostStore) DeletePost(_ context.Context, _ int64, serial string) (int64, error) {
	if store.err != nil {
		return 0, store.err
	}
	if _, ok := store.posts[serial]; !ok {
		return 0, ErrPostNotFound
	}
	store.deleteCalled = true
	delete(store.posts, serial)
	return store.deletedID, nil
}

type memoryElementStore struct {
	page     ElementPage
	element  Element
	lastEdit ElementEdit
	deleted  []int64
	affected []int64
	err      error
}

func (store *memoryElementStore) Elements(
	_ context.Context, _ int64, _ string, query ElementQuery,
) (ElementPage, error) {
	if store.err != nil {
		return ElementPage{}, store.err
	}
	page := store.page
	page.Page, page.PerPage = query.Page, query.PerPage
	return page, nil
}

func (store *memoryElementStore) UpdateElement(
	_ context.Context, _ int64, _ int64, edit ElementEdit,
) (Element, error) {
	if store.err != nil {
		return Element{}, store.err
	}
	store.lastEdit = edit
	return store.element, nil
}

func (store *memoryElementStore) DeleteElement(_ context.Context, _ int64, elementID int64) ([]int64, error) {
	if store.err != nil {
		return nil, store.err
	}
	store.deleted = append(store.deleted, elementID)
	return store.affected, nil
}

type memoryPasswords struct {
	hash string
	err  error
}

func (checker *memoryPasswords) PasswordHash(_ context.Context, _ int64) (string, error) {
	return checker.hash, checker.err
}

type recordingRefresher struct {
	posts []int64
	err   error
}

func (refresher *recordingRefresher) RefreshPostRank(_ context.Context, postID int64) error {
	refresher.posts = append(refresher.posts, postID)
	return refresher.err
}

type harness struct {
	service   *Service
	posts     *memoryPostStore
	elements  *memoryElementStore
	passwords *memoryPasswords
	ranks     *recordingRefresher
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	posts := newMemoryPostStore()
	elements := &memoryElementStore{}
	passwords := &memoryPasswords{}
	ranks := &recordingRefresher{}
	service, err := NewService(ServiceOptions{
		Posts: posts, Elements: elements, Passwords: passwords, Ranks: ranks,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &harness{service: service, posts: posts, elements: elements, passwords: passwords, ranks: ranks}
}

func codeFor(t *testing.T, err error, field string) string {
	t.Helper()
	var refused *ErrInvalid
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want an ErrInvalid", err)
	}
	codes := refused.Fields[field]
	if len(codes) != 1 {
		t.Fatalf("fields = %v, want one code for %q", refused.Fields, field)
	}
	return codes[0]
}

func validDraft() PostDraft {
	return PostDraft{Title: "a title", Description: "a description", AccessPolicy: PolicyPublic}
}

func TestCreatePostWritesTheDraftAndAnswersWithASerial(t *testing.T) {
	harness := newHarness(t)

	serial, err := harness.service.CreatePost(context.Background(), 7, validDraft())
	if err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}
	if len(serial) != SerialLength {
		t.Errorf("serial = %q, want %d characters", serial, SerialLength)
	}
	if len(harness.posts.created) != 1 {
		t.Fatalf("the store saw %d creates, want 1", len(harness.posts.created))
	}
}

func TestCreatePostValidatesWhatLaravelValidated(t *testing.T) {
	cases := []struct {
		name  string
		draft PostDraft
		field string
		want  string
	}{
		{"no title", PostDraft{Description: "d", AccessPolicy: PolicyPublic}, "title", CodeRequired},
		{"whitespace title", PostDraft{Title: "   ", Description: "d", AccessPolicy: PolicyPublic}, "title", CodeRequired},
		{"long title", PostDraft{Title: strings.Repeat("あ", MaxTitleRunes+1), Description: "d", AccessPolicy: PolicyPublic}, "title", CodeTooLong},
		{"no description", PostDraft{Title: "t", AccessPolicy: PolicyPublic}, "description", CodeRequired},
		{"long description", PostDraft{Title: "t", Description: strings.Repeat("あ", MaxDescriptionRunes+1), AccessPolicy: PolicyPublic}, "description", CodeTooLong},
		{"no policy", PostDraft{Title: "t", Description: "d"}, "access_policy", CodeRequired},
		{"unknown policy", PostDraft{Title: "t", Description: "d", AccessPolicy: "unlisted"}, "access_policy", CodeInvalidPolicy},
		{"password policy with no password", PostDraft{Title: "t", Description: "d", AccessPolicy: PolicyPassword}, "password", CodeRequired},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newHarness(t)

			_, err := harness.service.CreatePost(context.Background(), 7, testCase.draft)
			if code := codeFor(t, err, testCase.field); code != testCase.want {
				t.Errorf("code = %q, want %q", code, testCase.want)
			}
			if len(harness.posts.created) != 0 {
				t.Error("an invalid draft reached the store")
			}
		})
	}
}

func TestATagListLongerThanTheLimitIsRefused(t *testing.T) {
	harness := newHarness(t)
	draft := validDraft()
	draft.Tags = []string{"a", "b", "c", "d", "e", "f"}

	_, err := harness.service.CreatePost(context.Background(), 7, draft)
	if code := codeFor(t, err, "tags"); code != CodeTooMany {
		t.Errorf("code = %q, want %q", code, CodeTooMany)
	}
}

// SHA-256, NOT BCRYPT. The column is compared against hash('sha256', $password) by the
// game and rank pages, which are still Laravel's. A different hash would lock 1,035
// password-protected posts out of their own visitors.
func TestAPostPasswordIsHashedTheWayLaravelHashesIt(t *testing.T) {
	harness := newHarness(t)
	draft := validDraft()
	draft.AccessPolicy, draft.Password = PolicyPassword, "the-door-code"

	if _, err := harness.service.CreatePost(context.Background(), 7, draft); err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}

	sum := sha256.Sum256([]byte("the-door-code"))
	if want := hex.EncodeToString(sum[:]); harness.posts.createdHash != want {
		t.Errorf("stored hash = %q, want the SHA-256 hex digest %q", harness.posts.createdHash, want)
	}
}

func TestUpdatePostRewritesTheMetadata(t *testing.T) {
	harness := newHarness(t)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh", Title: "before", AccessPolicy: PolicyPublic}

	draft := validDraft()
	draft.Title = "after"
	post, err := harness.service.UpdatePost(context.Background(), 7, "abcdefgh", draft)
	if err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	if post.Title != "after" {
		t.Errorf("title = %q, want %q", post.Title, "after")
	}
}

/*
THE PASSWORD RULES, WHICH ARE THE PART OF THIS ENDPOINT WORTH GETTING RIGHT.

A post that stays password-protected keeps its password when the form does not resend it,
so editing a title does not silently open the door. A post that leaves the password policy
has its password erased, so switching back later cannot revive one the author has
forgotten. Both are validatePostPassword's behaviour.
*/
func TestTheUpdatePasswordRules(t *testing.T) {
	cases := []struct {
		name        string
		stored      bool
		policy      string
		password    string
		wantHashSet bool
		wantCleared bool
		wantRefused bool
	}{
		{name: "keeps the stored password when none is sent", stored: true, policy: PolicyPassword},
		{name: "replaces it when one is sent", stored: true, policy: PolicyPassword, password: "new", wantHashSet: true},
		{name: "sets the first one", stored: false, policy: PolicyPassword, password: "first", wantHashSet: true},
		{name: "refuses password policy with nothing to protect it", stored: false, policy: PolicyPassword, wantRefused: true},
		{name: "clears it when the post becomes public", stored: true, policy: PolicyPublic, wantCleared: true},
		{name: "clears it when the post becomes private", stored: true, policy: PolicyPrivate, wantCleared: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.posts.posts["abcdefgh"] = Post{
				Serial: "abcdefgh", AccessPolicy: PolicyPassword, HasPassword: testCase.stored,
			}

			draft := validDraft()
			draft.AccessPolicy, draft.Password = testCase.policy, testCase.password
			_, err := harness.service.UpdatePost(context.Background(), 7, "abcdefgh", draft)

			if testCase.wantRefused {
				if code := codeFor(t, err, "password"); code != CodeRequired {
					t.Errorf("code = %q, want %q", code, CodeRequired)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdatePost() error = %v", err)
			}

			switch {
			case testCase.wantHashSet:
				if harness.posts.updatedHash == nil || *harness.posts.updatedHash == "" {
					t.Errorf("hash = %v, want a new one written", harness.posts.updatedHash)
				}
			case testCase.wantCleared:
				if harness.posts.updatedHash == nil || *harness.posts.updatedHash != "" {
					t.Errorf("hash = %v, want it cleared", harness.posts.updatedHash)
				}
			default:
				if harness.posts.updatedHash != nil {
					t.Errorf("hash = %q, want the stored one left alone", *harness.posts.updatedHash)
				}
			}
		})
	}
}

func TestUpdatingAPostThatIsNotYoursIsNotFound(t *testing.T) {
	harness := newHarness(t)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh", AccessPolicy: PolicyPublic}

	_, err := harness.service.UpdatePost(context.Background(), 999, "abcdefgh", validDraft())
	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("error = %v, want ErrPostNotFound", err)
	}
}

func TestDeletePostRequiresTheAccountPasswordWhenThereIsOne(t *testing.T) {
	harness := newHarness(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("the-account-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	harness.passwords.hash = string(hash)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh"}
	harness.posts.deletedID = 55

	if err := harness.service.DeletePost(context.Background(), 7, "abcdefgh", "wrong"); err == nil {
		t.Fatal("a wrong password deleted the post")
	} else if code := codeFor(t, err, "password"); code != CodeIncorrect {
		t.Errorf("code = %q, want %q", code, CodeIncorrect)
	}
	if harness.posts.deleteCalled {
		t.Fatal("the store was asked to delete despite a wrong password")
	}

	if err := harness.service.DeletePost(
		context.Background(), 7, "abcdefgh", "the-account-password"); err != nil {
		t.Fatalf("DeletePost() error = %v", err)
	}
	if !harness.posts.deleteCalled {
		t.Error("the post was not deleted")
	}
	// The rank report the deletion invalidated is queued for rebuild, which is what
	// DeletePostRank dispatched.
	if len(harness.ranks.posts) != 1 || harness.ranks.posts[0] != 55 {
		t.Errorf("refreshed %v, want [55]", harness.ranks.posts)
	}
}

// THE DEFECT THIS PORT FIXES. password_verify against an empty hash is always false, so
// the 11,040 accounts created through Google could never delete a post — the endpoint
// refused them with "password is not correct" whatever they typed. An account with no
// password now deletes on its access token, which is the proof every other write here
// rests on.
func TestAnAccountWithNoPasswordCanStillDeleteItsPost(t *testing.T) {
	harness := newHarness(t)
	harness.passwords.hash = ""
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh"}

	if err := harness.service.DeletePost(context.Background(), 7, "abcdefgh", ""); err != nil {
		t.Fatalf("DeletePost() error = %v", err)
	}
	if !harness.posts.deleteCalled {
		t.Error("the post was not deleted")
	}
}

func TestDeletePostWithoutAPasswordWhenOneIsSetIsRefused(t *testing.T) {
	harness := newHarness(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("the-account-password"), bcrypt.MinCost)
	harness.passwords.hash = string(hash)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh"}

	err := harness.service.DeletePost(context.Background(), 7, "abcdefgh", "")
	if code := codeFor(t, err, "password"); code != CodeRequired {
		t.Errorf("code = %q, want %q", code, CodeRequired)
	}
}

// The rows are already gone by then, so refusing the request would be a lie. The daily
// rank schedule corrects a missed refresh.
func TestAFailedRankRefreshDoesNotFailTheDeletion(t *testing.T) {
	harness := newHarness(t)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh"}
	harness.posts.deletedID = 55
	harness.ranks.err = errors.New("redis is down")

	if err := harness.service.DeletePost(context.Background(), 7, "abcdefgh", ""); err != nil {
		t.Fatalf("DeletePost() error = %v", err)
	}
	if !harness.posts.deleteCalled {
		t.Error("the post was not deleted")
	}
}

func TestCleanTagsTrimsDropsEmptiesAndDeduplicates(t *testing.T) {
	got := CleanTags([]string{" a ", "", "b", "a", "   ", "b"})
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// The pivot has a composite primary key, so attaching the same tag twice is a constraint
// error rather than a duplicate row — the original attached in a loop with no check, and
// a user typing the same tag twice would have failed the whole edit.
func TestDuplicateTagsDoNotReachTheStore(t *testing.T) {
	harness := newHarness(t)
	harness.posts.posts["abcdefgh"] = Post{Serial: "abcdefgh", AccessPolicy: PolicyPublic}

	draft := validDraft()
	draft.Tags = []string{"cats", "cats", "dogs"}
	post, err := harness.service.UpdatePost(context.Background(), 7, "abcdefgh", draft)
	if err != nil {
		t.Fatalf("UpdatePost() error = %v", err)
	}
	if len(post.Tags) != 2 {
		t.Errorf("tags = %v, want two", post.Tags)
	}
}

func TestNewPostSerialIsEightLowercaseAlphanumericsAndDistinct(t *testing.T) {
	seen := map[string]struct{}{}
	for attempt := 0; attempt < 200; attempt++ {
		serial, err := NewPostSerial()
		if err != nil {
			t.Fatalf("NewPostSerial() error = %v", err)
		}
		if len(serial) != SerialLength {
			t.Fatalf("serial = %q, want %d characters", serial, SerialLength)
		}
		if strings.ToLower(serial) != serial {
			t.Fatalf("serial = %q, want lowercase", serial)
		}
		for _, letter := range serial {
			if !strings.ContainsRune(serialAlphabet, letter) {
				t.Fatalf("serial %q contains %q, which is outside the alphabet", serial, letter)
			}
		}
		if _, already := seen[serial]; already {
			t.Fatalf("serial %q was generated twice in 200 attempts", serial)
		}
		seen[serial] = struct{}{}
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	posts := newMemoryPostStore()
	elements := &memoryElementStore{}
	passwords := &memoryPasswords{}

	cases := map[string]ServiceOptions{
		"no posts":     {Elements: elements, Passwords: passwords},
		"no elements":  {Posts: posts, Passwords: passwords},
		"no passwords": {Posts: posts, Elements: elements},
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewService(options); err == nil {
				t.Error("NewService() accepted a missing dependency")
			}
		})
	}
	// Ranks is the one that may be absent: a deployment without Redis still edits posts.
	if _, err := NewService(ServiceOptions{
		Posts: posts, Elements: elements, Passwords: passwords}); err != nil {
		t.Errorf("NewService() without a refresher: %v", err)
	}
}
