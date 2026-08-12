package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ---------- fakes ----------

type fakeUsers struct {
	mu       sync.Mutex
	byEmail  map[string]Credentials
	byID     map[int64]Credentials
	emailErr error
	idErr    error
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byEmail: map[string]Credentials{}, byID: map[int64]Credentials{}}
}

func (users *fakeUsers) add(email string, credentials Credentials) {
	users.mu.Lock()
	defer users.mu.Unlock()
	users.byEmail[strings.ToLower(email)] = credentials
	users.byID[credentials.UserID] = credentials
}

func (users *fakeUsers) FindByEmail(_ context.Context, email string) (Credentials, error) {
	users.mu.Lock()
	defer users.mu.Unlock()
	if users.emailErr != nil {
		return Credentials{}, users.emailErr
	}
	credentials, ok := users.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return Credentials{}, ErrUserNotFound
	}
	return credentials, nil
}

func (users *fakeUsers) FindByID(_ context.Context, userID int64) (Credentials, error) {
	users.mu.Lock()
	defer users.mu.Unlock()
	if users.idErr != nil {
		return Credentials{}, users.idErr
	}
	credentials, ok := users.byID[userID]
	if !ok {
		return Credentials{}, ErrUserNotFound
	}
	return credentials, nil
}

// memoryRefreshStore reproduces the MySQL store's semantics, including the
// conditional MarkUsed that makes concurrent rotation safe. Anything looser would let
// these tests pass while the real store failed.
type memoryRefreshStore struct {
	mu        sync.Mutex
	nextID    int64
	byHash    map[string]*Session
	createErr error
	usedErr   error
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{nextID: 1, byHash: map[string]*Session{}}
}

func (store *memoryRefreshStore) Create(_ context.Context, record NewSession) (Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.createErr != nil {
		return Session{}, store.createErr
	}
	session := &Session{
		ID: store.nextID, UserID: record.UserID, FamilyID: record.FamilyID,
		IssuedAt: record.IssuedAt, ExpiresAt: record.ExpiresAt, CSRFHash: record.CSRFHash,
	}
	store.nextID++
	store.byHash[record.TokenHash] = session
	return *session, nil
}

func (store *memoryRefreshStore) FindByTokenHash(_ context.Context, tokenHash string) (Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.byHash[tokenHash]
	if !ok {
		return Session{}, ErrRefreshTokenInvalid
	}
	return *session, nil
}

func (store *memoryRefreshStore) MarkUsed(_ context.Context, sessionID int64, usedAt time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.usedErr != nil {
		return store.usedErr
	}
	for _, session := range store.byHash {
		if session.ID != sessionID {
			continue
		}
		// The WHERE used_at IS NULL of the real statement.
		if session.UsedAt != nil {
			return ErrRefreshTokenReused
		}
		stamp := usedAt
		session.UsedAt = &stamp
		return nil
	}
	return ErrRefreshTokenInvalid
}

func (store *memoryRefreshStore) RevokeFamily(_ context.Context, familyID string, revokedAt time.Time) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var count int64
	for _, session := range store.byHash {
		if session.FamilyID == familyID && session.RevokedAt == nil {
			stamp := revokedAt
			session.RevokedAt = &stamp
			count++
		}
	}
	return count, nil
}

func (store *memoryRefreshStore) RevokeUser(_ context.Context, userID int64, revokedAt time.Time) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var count int64
	for _, session := range store.byHash {
		if session.UserID == userID && session.RevokedAt == nil {
			stamp := revokedAt
			session.RevokedAt = &stamp
			count++
		}
	}
	return count, nil
}

func (store *memoryRefreshStore) DeleteExpired(_ context.Context, before time.Time, limit int) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var count int64
	for hash, session := range store.byHash {
		if count >= int64(limit) {
			break
		}
		if session.ExpiresAt.Before(before) {
			delete(store.byHash, hash)
			count++
		}
	}
	return count, nil
}

func (store *memoryRefreshStore) expire(tokenHash string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if session, ok := store.byHash[tokenHash]; ok {
		session.ExpiresAt = session.IssuedAt.Add(-time.Second)
	}
}

// ---------- harness ----------

const (
	testEmail    = "player@example.test"
	testPassword = "s3cret-probe-password"
)

type authHarness struct {
	service *Service
	users   *fakeUsers
	store   *memoryRefreshStore
	now     time.Time
}

func newAuthHarness(t *testing.T) *authHarness {
	t.Helper()

	users := newFakeUsers()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	users.add(testEmail, Credentials{UserID: 42, PasswordHash: string(hash), Roles: []string{}})

	store := newMemoryRefreshStore()
	private, _, err := generateTestKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "2pick-go-api",
		PrivateKey: private, TTL: DefaultAccessTokenTTL, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	service, err := NewService(ServiceOptions{
		Users: users, Sessions: store, Issuer: issuer,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &authHarness{service: service, users: users, store: store, now: now}
}

func (h *authHarness) login(t *testing.T) Grant {
	t.Helper()
	grant, err := h.service.Login(context.Background(), testEmail, testPassword, ClientInfo{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return grant
}

// ---------- login ----------

func TestLoginIssuesAnAccessTokenAndASession(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)

	if grant.UserID != 42 {
		t.Errorf("user id = %d, want 42", grant.UserID)
	}
	if grant.Access.Token == "" || grant.Access.TokenType != "Bearer" {
		t.Errorf("access token = %+v", grant.Access)
	}
	if grant.Refresh.Token == "" || grant.Refresh.CSRFToken == "" || grant.Refresh.FamilyID == "" {
		t.Errorf("refresh = %+v", grant.Refresh)
	}
	// The token itself must never be what is stored.
	if _, err := h.store.FindByTokenHash(context.Background(), grant.Refresh.Token); err == nil {
		t.Fatal("the raw refresh token is a valid lookup key; only its hash may be stored")
	}
	if _, err := h.store.FindByTokenHash(context.Background(), HashToken(grant.Refresh.Token)); err != nil {
		t.Fatalf("the hashed token should resolve: %v", err)
	}
}

// THE 11,040 ACCOUNTS. Most production accounts signed in through Google or Twitch
// and their password column is an empty string. Any of these succeeding would be an
// account takeover for 82% of the user base.
func TestLoginRejectsAccountsWithNoPassword(t *testing.T) {
	h := newAuthHarness(t)
	h.users.add("oauth@example.test", Credentials{UserID: 99, PasswordHash: "", Roles: []string{}})

	for _, password := range []string{"", " ", "anything", "$2y$10$", testPassword} {
		_, err := h.service.Login(context.Background(), "oauth@example.test", password, ClientInfo{})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("password %q against an empty hash returned %v, want ErrInvalidCredentials", password, err)
		}
	}

	// Whitespace-only hashes count as absent too: a column trimmed to spaces by some
	// earlier migration must not become a bypass.
	h.users.add("blank@example.test", Credentials{UserID: 100, PasswordHash: "   ", Roles: []string{}})
	if _, err := h.service.Login(context.Background(), "blank@example.test", "x", ClientInfo{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("a whitespace hash returned %v", err)
	}
}

// Every failure reports the same error. A distinguishable "no such account" turns the
// login form into an account enumeration oracle.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	h := newAuthHarness(t)

	cases := map[string][2]string{
		"unknown address":  {"nobody@example.test", testPassword},
		"wrong password":   {testEmail, "not-the-password"},
		"empty password":   {testEmail, ""},
		"empty address":    {"", testPassword},
		"oversized email":  {strings.Repeat("a", MaxEmailBytes+1) + "@x.test", testPassword},
		"oversized secret": {testEmail, strings.Repeat("p", MaxPasswordBytes+1)},
	}
	for name, pair := range cases {
		_, err := h.service.Login(context.Background(), pair[0], pair[1], ClientInfo{})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: error = %v, want ErrInvalidCredentials", name, err)
		}
	}
}

// A database failure must not be reported as bad credentials: that would tell a user
// their password is wrong during an outage, and hide the outage from the operator.
func TestLoginSurfacesStoreFailures(t *testing.T) {
	h := newAuthHarness(t)
	h.users.emailErr = errors.New("connection refused")

	_, err := h.service.Login(context.Background(), testEmail, testPassword, ClientInfo{})
	if err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error = %v, want the underlying failure", err)
	}
}

// ---------- refresh ----------

func TestRefreshRotatesTheSession(t *testing.T) {
	h := newAuthHarness(t)
	first := h.login(t)

	second, err := h.service.Refresh(context.Background(),
		first.Refresh.Token, first.Refresh.CSRFToken, ClientInfo{})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if second.Refresh.Token == first.Refresh.Token {
		t.Error("the refresh token was not rotated")
	}
	if second.Refresh.CSRFToken == first.Refresh.CSRFToken {
		t.Error("the csrf token was not rotated")
	}
	// Same family: rotation continues one login rather than starting a new one, which
	// is what makes replay detection able to revoke the whole chain.
	if second.Refresh.FamilyID != first.Refresh.FamilyID {
		t.Errorf("family changed: %q -> %q", first.Refresh.FamilyID, second.Refresh.FamilyID)
	}
}

// The cookie arrives on a cross-site request automatically, so possession of it
// cannot be the only thing authorising a rotation.
func TestRefreshRequiresTheCSRFToken(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)

	for name, presented := range map[string]string{
		"absent":     "",
		"wrong":      "not-the-csrf-token",
		"the cookie": grant.Refresh.Token, // the refresh token is not the CSRF token
	} {
		_, err := h.service.Refresh(context.Background(), grant.Refresh.Token, presented, ClientInfo{})
		if !errors.Is(err, ErrCSRFMismatch) {
			t.Errorf("%s csrf: error = %v, want ErrCSRFMismatch", name, err)
		}
	}

	// And a failed CSRF check must not have consumed the token.
	if _, err := h.service.Refresh(context.Background(),
		grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{}); err != nil {
		t.Fatalf("the session should still be usable after a CSRF failure: %v", err)
	}
}

// THE THEFT RESPONSE. A used token presented again means someone holds a copy. There
// is no way to tell the attacker from the victim, so the whole chain dies.
func TestReplayingATokenRevokesTheWholeFamily(t *testing.T) {
	h := newAuthHarness(t)
	first := h.login(t)

	second, err := h.service.Refresh(context.Background(),
		first.Refresh.Token, first.Refresh.CSRFToken, ClientInfo{})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// The attacker replays the token the victim already spent.
	_, err = h.service.Refresh(context.Background(),
		first.Refresh.Token, first.Refresh.CSRFToken, ClientInfo{})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("replay error = %v, want ErrRefreshTokenReused", err)
	}

	// And the victim's current token is dead too — that is the point.
	if _, err := h.service.Refresh(context.Background(),
		second.Refresh.Token, second.Refresh.CSRFToken, ClientInfo{}); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("the live token survived a replay: %v", err)
	}
}

// Two rotations racing on one token: exactly one may win. The loser is a replay.
func TestConcurrentRefreshAllowsOnlyOne(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)

	type outcome struct{ err error }
	results := make(chan outcome, 2)
	var start sync.WaitGroup
	start.Add(1)
	for index := 0; index < 2; index++ {
		go func() {
			start.Wait()
			_, err := h.service.Refresh(context.Background(),
				grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{})
			results <- outcome{err: err}
		}()
	}
	start.Done()

	var succeeded, failed int
	for index := 0; index < 2; index++ {
		if (<-results).err == nil {
			succeeded++
		} else {
			failed++
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("%d succeeded and %d failed; exactly one rotation may win", succeeded, failed)
	}
}

func TestRefreshRejectsExpiredAndUnknownTokens(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)

	if _, err := h.service.Refresh(context.Background(), "not-a-token", "x", ClientInfo{}); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Errorf("unknown token: error = %v", err)
	}
	if _, err := h.service.Refresh(context.Background(), "", "x", ClientInfo{}); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Errorf("empty token: error = %v", err)
	}

	h.store.expire(HashToken(grant.Refresh.Token))
	if _, err := h.service.Refresh(context.Background(),
		grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{}); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Errorf("expired token: error = %v", err)
	}
}

// Roles are re-read on every rotation, so a ban applied mid-session takes effect at
// the next refresh instead of at the next login.
func TestRefreshRereadsRoles(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)
	if len(grant.Roles) != 0 {
		t.Fatalf("roles = %v, want none at login", grant.Roles)
	}

	h.users.add(testEmail, Credentials{
		UserID: 42, PasswordHash: h.users.byID[42].PasswordHash, Roles: []string{"banned"},
	})

	rotated, err := h.service.Refresh(context.Background(),
		grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(rotated.Roles) != 1 || rotated.Roles[0] != "banned" {
		t.Fatalf("roles = %v, want [banned] after the role was added", rotated.Roles)
	}
}

// A deleted account cannot be refreshed back into a session.
func TestRefreshFailsWhenTheAccountIsGone(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)
	h.users.idErr = ErrUserNotFound

	if _, err := h.service.Refresh(context.Background(),
		grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{}); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("error = %v, want ErrRefreshTokenInvalid", err)
	}
}

// ---------- logout and revocation ----------

func TestLogoutRevokesTheFamily(t *testing.T) {
	h := newAuthHarness(t)
	first := h.login(t)
	second, err := h.service.Refresh(context.Background(),
		first.Refresh.Token, first.Refresh.CSRFToken, ClientInfo{})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if err := h.service.Logout(context.Background(), second.Refresh.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := h.service.Refresh(context.Background(),
		second.Refresh.Token, second.Refresh.CSRFToken, ClientInfo{}); err == nil {
		t.Fatal("the session survived logout")
	}
}

// Logging out a token the server has never seen is what a client with a stale cookie
// does. Reporting an error there would leave it unable to clear its own state.
func TestLogoutOfAnUnknownTokenSucceeds(t *testing.T) {
	h := newAuthHarness(t)
	if err := h.service.Logout(context.Background(), "never-issued"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if err := h.service.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout(\"\") error = %v", err)
	}
}

// RevokeAll is the ban and password-change path: every device, not just one.
func TestRevokeAllEndsEverySession(t *testing.T) {
	h := newAuthHarness(t)
	first := h.login(t)
	second := h.login(t)
	if first.Refresh.FamilyID == second.Refresh.FamilyID {
		t.Fatal("two logins must start two families")
	}

	revoked, err := h.service.RevokeAll(context.Background(), 42)
	if err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	if revoked != 2 {
		t.Errorf("revoked %d sessions, want 2", revoked)
	}
	for _, grant := range []Grant{first, second} {
		if _, err := h.service.Refresh(context.Background(),
			grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{}); err == nil {
			t.Error("a session survived RevokeAll")
		}
	}
}

func TestVerifyCSRFMatchesTheRefreshPath(t *testing.T) {
	h := newAuthHarness(t)
	grant := h.login(t)

	if err := h.service.VerifyCSRF(context.Background(), grant.Refresh.Token, grant.Refresh.CSRFToken); err != nil {
		t.Fatalf("VerifyCSRF() error = %v", err)
	}
	if err := h.service.VerifyCSRF(context.Background(), grant.Refresh.Token, "wrong"); !errors.Is(err, ErrCSRFMismatch) {
		t.Errorf("wrong csrf: error = %v", err)
	}
	if err := h.service.VerifyCSRF(context.Background(), "unknown", "x"); !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Errorf("unknown token: error = %v", err)
	}
	// Checking CSRF must not consume the session.
	if _, err := h.service.Refresh(context.Background(),
		grant.Refresh.Token, grant.Refresh.CSRFToken, ClientInfo{}); err != nil {
		t.Fatalf("VerifyCSRF consumed the token: %v", err)
	}
}

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	private, _, _ := generateTestKeypair()
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "aud", PrivateKey: private,
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	complete := ServiceOptions{Users: newFakeUsers(), Sessions: newMemoryRefreshStore(), Issuer: issuer}

	for name, mutate := range map[string]func(*ServiceOptions){
		"no users":    func(o *ServiceOptions) { o.Users = nil },
		"no sessions": func(o *ServiceOptions) { o.Sessions = nil },
		"no issuer":   func(o *ServiceOptions) { o.Issuer = nil },
	} {
		options := complete
		mutate(&options)
		if _, err := NewService(options); err == nil {
			t.Errorf("NewService() should reject the %q case", name)
		}
	}
	if _, err := NewService(complete); err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
}

// ---------- token and csrf primitives ----------

func TestRefreshTokensAreUnpredictableAndHashedOneWay(t *testing.T) {
	seen := make(map[string]struct{}, 200)
	for index := 0; index < 200; index++ {
		token, hash, err := NewRefreshToken()
		if err != nil {
			t.Fatalf("NewRefreshToken() error = %v", err)
		}
		if token == hash {
			t.Fatal("the stored hash equals the token")
		}
		if hash != HashToken(token) {
			t.Fatal("HashToken does not reproduce the stored hash")
		}
		if _, duplicate := seen[token]; duplicate {
			t.Fatal("a refresh token repeated")
		}
		seen[token] = struct{}{}
	}
}

func TestCheckCSRFRejectsEmptyValues(t *testing.T) {
	token, hash, err := NewCSRFToken()
	if err != nil {
		t.Fatalf("NewCSRFToken() error = %v", err)
	}
	if err := CheckCSRF(token, hash); err != nil {
		t.Fatalf("a matching pair was rejected: %v", err)
	}
	// An empty presented value against an empty stored hash must never pass: that is
	// the shape a session row with no CSRF would have.
	for name, pair := range map[string][2]string{
		"both empty":      {"", ""},
		"empty presented": {"", hash},
		"empty stored":    {token, ""},
		"mismatch":        {token, HashToken("other")},
	} {
		if err := CheckCSRF(pair[0], pair[1]); !errors.Is(err, ErrCSRFMismatch) {
			t.Errorf("%s: error = %v, want ErrCSRFMismatch", name, err)
		}
	}
}

func TestSubjectToUserID(t *testing.T) {
	if id, err := SubjectToUserID(" 42 "); err != nil || id != 42 {
		t.Errorf("SubjectToUserID(\" 42 \") = %d, %v", id, err)
	}
	for _, subject := range []string{"", "0", "-1", "abc", "4.2"} {
		if _, err := SubjectToUserID(subject); err == nil {
			t.Errorf("SubjectToUserID(%q) should fail", subject)
		}
	}
}
