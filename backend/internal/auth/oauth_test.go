package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// memorySocialStore stands in for MySQLSocialStore. The unique-index behaviour it has
// to reproduce is the one that matters: a second link for the same subject fails.
type memorySocialStore struct {
	mutex sync.Mutex
	// bySubject maps a provider subject to a user id.
	bySubject map[string]int64
	// emails is every address held by an account, however it signed up.
	emails map[string]bool
	// links records which users already have a provider account attached.
	links   map[int64]string
	roles   map[int64][]string
	nextID  int64
	created []NewLinkedUser
	// failCreate makes the next CreateLinkedUser fail, for the concurrent-signup path.
	failCreate error
}

func newMemorySocialStore() *memorySocialStore {
	return &memorySocialStore{
		bySubject: map[string]int64{},
		emails:    map[string]bool{},
		links:     map[int64]string{},
		roles:     map[int64][]string{},
		nextID:    100,
	}
}

func (store *memorySocialStore) FindByProviderSubject(
	_ context.Context, _, subject string,
) (Credentials, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	userID, found := store.bySubject[subject]
	if !found {
		return Credentials{}, ErrUserNotFound
	}
	roles := store.roles[userID]
	if roles == nil {
		roles = []string{}
	}
	return Credentials{UserID: userID, Roles: roles}, nil
}

func (store *memorySocialStore) EmailExists(_ context.Context, email string) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return store.emails[strings.ToLower(strings.TrimSpace(email))], nil
}

func (store *memorySocialStore) CreateLinkedUser(
	_ context.Context, record NewLinkedUser,
) (Credentials, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.failCreate != nil {
		failure := store.failCreate
		store.failCreate = nil
		return Credentials{}, failure
	}
	if _, taken := store.bySubject[record.Subject]; taken {
		return Credentials{}, ErrOAuthAlreadyLinked
	}
	if store.emails[strings.ToLower(record.Email)] {
		return Credentials{}, ErrOAuthEmailTaken
	}
	store.nextID++
	userID := store.nextID
	store.bySubject[record.Subject] = userID
	store.emails[strings.ToLower(record.Email)] = true
	store.links[userID] = record.Subject
	store.created = append(store.created, record)
	return Credentials{UserID: userID, Roles: []string{}}, nil
}

func (store *memorySocialStore) Link(_ context.Context, request LinkRequest) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, taken := store.bySubject[request.Subject]; taken {
		return ErrOAuthAlreadyLinked
	}
	if _, held := store.links[request.UserID]; held {
		return ErrOAuthAlreadyLinked
	}
	store.bySubject[request.Subject] = request.UserID
	store.links[request.UserID] = request.Subject
	return nil
}

// fakeProvider records what it was asked and returns a fixed identity.
type fakeProvider struct {
	identity      OAuthIdentity
	err           error
	exchanges     int
	lastCode      string
	lastVerifier  string
	lastState     string
	lastChallenge string
}

func (provider *fakeProvider) Name() string { return ProviderGoogle }

func (provider *fakeProvider) AuthorizationURL(state, codeChallenge string) string {
	provider.lastState, provider.lastChallenge = state, codeChallenge
	return "https://provider.test/authorize?state=" + url.QueryEscape(state) +
		"&code_challenge=" + url.QueryEscape(codeChallenge)
}

func (provider *fakeProvider) Exchange(
	_ context.Context, code, verifier string,
) (OAuthIdentity, error) {
	provider.exchanges++
	provider.lastCode, provider.lastVerifier = code, verifier
	if provider.err != nil {
		return OAuthIdentity{}, provider.err
	}
	return provider.identity, nil
}

type oauthFixture struct {
	service  *OAuthService
	provider *fakeProvider
	social   *memorySocialStore
	states   *MemoryOAuthStates
	sessions *memoryRefreshStore
}

func newOAuthFixture(t *testing.T, identity OAuthIdentity) *oauthFixture {
	t.Helper()

	// The same harness the password tests use, so an OAuth login and a password login
	// are proven to issue the same kind of session rather than two that merely look
	// alike.
	harness := newAuthHarness(t)

	provider := &fakeProvider{identity: identity}
	social := newMemorySocialStore()
	states := NewMemoryOAuthStates()
	service, err := NewOAuthService(OAuthServiceOptions{
		Provider:        provider,
		States:          states,
		Social:          social,
		Sessions:        harness.service,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReturnAllowlist: []string{"http://localhost:4173/"},
		DefaultReturnTo: "http://localhost:4173/",
	})
	if err != nil {
		t.Fatalf("build the oauth service: %v", err)
	}
	return &oauthFixture{
		service: service, provider: provider, social: social,
		states: states, sessions: harness.store,
	}
}

func verifiedIdentity() OAuthIdentity {
	return OAuthIdentity{
		Subject:       "google-subject-1",
		Email:         "player@example.test",
		EmailVerified: true,
		Name:          "Player One",
		AvatarURL:     "https://example.test/avatar.png",
	}
}

// start runs a flow and returns the state key the provider was handed.
func (fixture *oauthFixture) start(t *testing.T, returnTo string, connectUserID int64) string {
	t.Helper()
	flow, err := fixture.service.Start(context.Background(), returnTo, connectUserID)
	if err != nil {
		t.Fatalf("start the flow: %v", err)
	}
	if flow.State == "" {
		t.Fatal("the flow has no state; the callback would have nothing to match")
	}
	return flow.State
}

// THE PKCE CHALLENGE MUST BE THE HASH, NOT THE VERIFIER. Sending the verifier as the
// challenge is a plausible-looking mistake that silently removes the protection: the
// exchange still succeeds, and anyone who captured the redirect can now spend the code.
func TestStartSendsTheHashedVerifierNotTheVerifier(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	stored, err := fixture.states.Consume(context.Background(), ProviderGoogle+":"+stateKey)
	if err != nil {
		t.Fatalf("the state was not stored under the provider namespace: %v", err)
	}
	if stored.Verifier == "" {
		t.Fatal("no verifier was stored; the exchange would have nothing to prove")
	}
	if fixture.provider.lastChallenge == stored.Verifier {
		t.Fatal("the challenge equals the verifier: this is plain PKCE, which proves nothing")
	}

	sum := sha256.Sum256([]byte(stored.Verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); fixture.provider.lastChallenge != want {
		t.Errorf("challenge = %q, want the S256 of the verifier %q",
			fixture.provider.lastChallenge, want)
	}
}

// A state is one-shot. If it were not, an authorization code captured from the redirect
// could be presented again to run the whole flow a second time.
func TestAStateCanOnlyBeUsedOnce(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	if _, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{}); err != nil {
		t.Fatalf("first callback: %v", err)
	}
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("second callback error = %v, want ErrOAuthStateInvalid", err)
	}
	if fixture.provider.exchanges != 1 {
		t.Errorf("the provider was called %d times, want 1: a replayed state must not reach it",
			fixture.provider.exchanges)
	}
}

func TestAnUnknownStateNeverReachesTheProvider(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())

	for name, state := range map[string]string{
		"empty":   "",
		"unknown": "a-state-that-was-never-issued",
	} {
		_, err := fixture.service.Complete(context.Background(), state, "code", ClientInfo{})
		if !errors.Is(err, ErrOAuthStateInvalid) {
			t.Errorf("%s state: error = %v, want ErrOAuthStateInvalid", name, err)
		}
	}
	if fixture.provider.exchanges != 0 {
		t.Errorf("the provider was called %d times for states that do not exist",
			fixture.provider.exchanges)
	}
}

// A state started for a login must not be usable to run a link, and the namespace is
// what enforces it. Consuming with the bare key has to miss.
func TestStateKeysAreNamespacedByProvider(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	if _, err := fixture.states.Consume(context.Background(), stateKey); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("the bare key resolved; the state is not namespaced (err = %v)", err)
	}
}

func TestCompleteMissingCodeIsRejected(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	_, err := fixture.service.Complete(context.Background(), stateKey, "", ClientInfo{})
	if !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("error = %v, want ErrOAuthStateInvalid", err)
	}
	if fixture.provider.exchanges != 0 {
		t.Error("the provider was called with no code")
	}
}

func TestFirstSignInCreatesAnAccountAndASession(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	completed, err := fixture.service.Complete(context.Background(), stateKey, "code-1",
		ClientInfo{IP: "203.0.113.7", UserAgent: "probe/1.0"})
	if err != nil {
		t.Fatalf("complete the flow: %v", err)
	}
	if !completed.Created {
		t.Error("Created is false; the caller cannot tell a new account from a returning one")
	}
	if completed.Linked {
		t.Error("Linked is true on a login")
	}
	if completed.Grant.Access.Token == "" || completed.Grant.Refresh.Token == "" {
		t.Fatal("no session was issued")
	}
	if completed.UserID == 0 {
		t.Error("no user id was reported")
	}

	if len(fixture.social.created) != 1 {
		t.Fatalf("%d accounts were created, want 1", len(fixture.social.created))
	}
	created := fixture.social.created[0]
	if created.Subject != "google-subject-1" || created.Email != "player@example.test" {
		t.Errorf("the account was created from the wrong identity: %+v", created)
	}
	if created.Name != "Player One" {
		t.Errorf("name = %q", created.Name)
	}
	if !created.EmailVerified {
		t.Error("the provider verified the address but it was stored as unverified")
	}
}

// A returning user is found by subject, not by address. This is the case that breaks if
// the lookup order is reversed: someone who changed their Google address, or whose
// address now collides with another account, must still sign in.
func TestAReturningUserIsFoundBySubjectEvenWhenTheAddressIsTaken(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())

	// First sign-in creates the account.
	first := fixture.start(t, "", 0)
	initial, err := fixture.service.Complete(context.Background(), first, "code-1", ClientInfo{})
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	// Their Google address now points somewhere that another account already holds.
	fixture.provider.identity.Email = "someone-else@example.test"
	fixture.social.emails["someone-else@example.test"] = true

	second := fixture.start(t, "", 0)
	again, err := fixture.service.Complete(context.Background(), second, "code-2", ClientInfo{})
	if err != nil {
		t.Fatalf("the returning user was refused: %v", err)
	}
	if again.UserID != initial.UserID {
		t.Errorf("signed in as %d, want the original %d", again.UserID, initial.UserID)
	}
	if again.Created {
		t.Error("a second account was created for a user who already had one")
	}
	if len(fixture.social.created) != 1 {
		t.Errorf("%d accounts exist, want 1", len(fixture.social.created))
	}
}

// Refusing rather than linking is deliberate and matches Laravel. Linking on a matching
// address would hand the local account to whoever controls the provider account.
func TestAnAddressThatAlreadyHasAnAccountIsRefused(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	fixture.social.emails["player@example.test"] = true

	stateKey := fixture.start(t, "", 0)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthEmailTaken) {
		t.Fatalf("error = %v, want ErrOAuthEmailTaken", err)
	}
	if len(fixture.social.created) != 0 {
		t.Error("an account was created despite the refusal")
	}
}

// An unverified address must not match or create anything: the provider is passing
// along a string the user typed, and matching on it would be an account takeover.
func TestAnUnverifiedAddressIsRefused(t *testing.T) {
	identity := verifiedIdentity()
	identity.EmailVerified = false
	fixture := newOAuthFixture(t, identity)

	stateKey := fixture.start(t, "", 0)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthEmailUnverified) {
		t.Fatalf("error = %v, want ErrOAuthEmailUnverified", err)
	}
	if len(fixture.social.created) != 0 {
		t.Error("an account was created from an unverified address")
	}
}

func TestAnEmptyAddressIsRefused(t *testing.T) {
	identity := verifiedIdentity()
	identity.Email = "   "
	fixture := newOAuthFixture(t, identity)

	stateKey := fixture.start(t, "", 0)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthEmailUnverified) {
		t.Fatalf("error = %v, want ErrOAuthEmailUnverified", err)
	}
}

// Two people clicking the same consent button at once: one insert wins the unique
// index, and the loser must still end up signed in rather than being told their brand
// new account is "already linked".
func TestASimultaneousFirstSignInStillSignsIn(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())

	// The other flow got there first: the row exists, and this create loses.
	fixture.social.bySubject["google-subject-1"] = 4242
	fixture.social.failCreate = ErrOAuthAlreadyLinked

	stateKey := fixture.start(t, "", 0)
	completed, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if err != nil {
		t.Fatalf("the losing flow failed instead of signing in: %v", err)
	}
	if completed.UserID != 4242 {
		t.Errorf("signed in as %d, want the account the winner created (4242)", completed.UserID)
	}
	if completed.Grant.Access.Token == "" {
		t.Error("no session was issued")
	}
}

// A link must not mint a session. The caller was already authenticated when the flow
// started; issuing a new grant here would let the callback escalate a link into a
// login.
func TestALinkIssuesNoSession(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 7)

	completed, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if err != nil {
		t.Fatalf("complete the link: %v", err)
	}
	if !completed.Linked {
		t.Error("Linked is false")
	}
	if completed.Grant.Access.Token != "" || completed.Grant.Refresh.Token != "" {
		t.Error("a link issued a session; it must not change who is signed in")
	}
	if completed.UserID != 7 {
		t.Errorf("user id = %d, want the user who started the link (7)", completed.UserID)
	}
	if fixture.social.links[7] != "google-subject-1" {
		t.Errorf("the link was not recorded: %v", fixture.social.links)
	}
}

func TestLinkingAProviderAccountTwiceIsRefused(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	// Already attached to somebody else.
	fixture.social.bySubject["google-subject-1"] = 99

	stateKey := fixture.start(t, "", 7)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthAlreadyLinked) {
		t.Fatalf("error = %v, want ErrOAuthAlreadyLinked", err)
	}
}

// A provider failure must not be reported as a user error, and its detail must not
// escape: the token endpoint's message contains the request this server made.
func TestAProviderFailureIsWrappedNotSurfaced(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	fixture.provider.err = errors.New("token endpoint returned 400: invalid_grant")

	stateKey := fixture.start(t, "", 0)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthProviderFailed) {
		t.Fatalf("error = %v, want it to wrap ErrOAuthProviderFailed", err)
	}
}

func TestAProviderThatReturnsNoSubjectIsRejected(t *testing.T) {
	identity := verifiedIdentity()
	identity.Subject = ""
	fixture := newOAuthFixture(t, identity)

	stateKey := fixture.start(t, "", 0)
	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthProviderFailed) {
		t.Fatalf("error = %v, want ErrOAuthProviderFailed", err)
	}
	if len(fixture.social.created) != 0 {
		t.Error("an account was created with no provider subject")
	}
}

// THE OPEN REDIRECT. The callback URL is one a user follows immediately after signing
// in, which makes it worth stealing. Anything not on the allowlist must collapse to the
// default rather than being honoured.
func TestReturnTargetsOutsideTheAllowlistAreIgnored(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())

	const allowed = "http://localhost:4173/"
	cases := map[string]string{
		"":                              allowed,
		"http://localhost:4173/":        "http://localhost:4173/",
		"http://localhost:4173/profile": "http://localhost:4173/profile",
		// Another origin outright.
		"https://evil.test/steal": allowed,
		// Protocol-relative: the browser reads //evil.test as an absolute URL.
		"//evil.test/steal": allowed,
		// The allowed origin as a prefix of a hostile one — the classic
		// startsWith mistake, caught here because the allowlist entry ends in "/".
		"http://localhost:4173.evil.test/": allowed,
		// A scheme that would execute script if followed.
		"javascript:alert(1)": allowed,
	}

	for candidate, want := range cases {
		stateKey := fixture.start(t, candidate, 0)
		completed, err := fixture.service.Complete(
			context.Background(), stateKey, "code-"+candidate, ClientInfo{})
		if err != nil {
			t.Fatalf("candidate %q: %v", candidate, err)
		}
		if completed.ReturnTo != want {
			t.Errorf("candidate %q returned to %q, want %q", candidate, completed.ReturnTo, want)
		}
		// Every candidate signs in the same returning user after the first.
		fixture.social.emails["player@example.test"] = false
	}
}

func TestAnExpiredStateIsRejected(t *testing.T) {
	fixture := newOAuthFixture(t, verifiedIdentity())
	stateKey := fixture.start(t, "", 0)

	// Wind the store's clock past the TTL.
	fixture.states.now = func() time.Time { return time.Now().Add(OAuthStateTTL + time.Minute) }

	_, err := fixture.service.Complete(context.Background(), stateKey, "code-1", ClientInfo{})
	if !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("error = %v, want ErrOAuthStateInvalid", err)
	}
}

// The name column is utf8mb4 and most of these accounts are Chinese. A byte limit would
// cut a character in half and MySQL would reject the row, so the limit is in runes.
func TestDisplayNameIsTruncatedInRunesNotBytes(t *testing.T) {
	cases := []struct {
		name     string
		identity OAuthIdentity
		want     string
	}{
		{
			name:     "short name is kept",
			identity: OAuthIdentity{Name: "Player One"},
			want:     "Player One",
		},
		{
			name:     "long ascii name is cut at twenty",
			identity: OAuthIdentity{Name: strings.Repeat("a", 30)},
			want:     strings.Repeat("a", 20),
		},
		{
			name: "twenty five chinese characters become twenty",
			// 25 runes, 75 bytes. A 20-byte cut would land mid-character.
			identity: OAuthIdentity{Name: strings.Repeat("排", 25)},
			want:     strings.Repeat("排", 20),
		},
		{
			name:     "no name falls back to the local part",
			identity: OAuthIdentity{Email: "player@example.test"},
			want:     "player",
		},
		{
			name:     "nothing at all still produces a name",
			identity: OAuthIdentity{},
			want:     "user",
		},
		{
			name:     "whitespace is not a name",
			identity: OAuthIdentity{Name: "   ", Email: "someone@example.test"},
			want:     "someone",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := DisplayNameFromIdentity(testCase.identity)
			if got != testCase.want {
				t.Errorf("name = %q, want %q", got, testCase.want)
			}
			if len([]rune(got)) > MaxDisplayNameRunes {
				t.Errorf("name is %d runes, over the %d the column allows",
					len([]rune(got)), MaxDisplayNameRunes)
			}
		})
	}
}

func TestNewOAuthServiceRejectsMissingDependencies(t *testing.T) {
	valid := OAuthServiceOptions{
		Provider:        &fakeProvider{},
		States:          NewMemoryOAuthStates(),
		Social:          newMemorySocialStore(),
		Sessions:        &Service{},
		DefaultReturnTo: "http://localhost:4173/",
	}

	cases := map[string]func(*OAuthServiceOptions){
		"no provider":       func(o *OAuthServiceOptions) { o.Provider = nil },
		"no state store":    func(o *OAuthServiceOptions) { o.States = nil },
		"no social store":   func(o *OAuthServiceOptions) { o.Social = nil },
		"no sessions":       func(o *OAuthServiceOptions) { o.Sessions = nil },
		"no return default": func(o *OAuthServiceOptions) { o.DefaultReturnTo = "" },
	}
	for name, remove := range cases {
		options := valid
		remove(&options)
		if _, err := NewOAuthService(options); err == nil {
			t.Errorf("%s: NewOAuthService succeeded", name)
		}
	}
}
