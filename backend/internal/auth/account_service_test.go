package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// The account rules, against an in-memory store. The MySQL half — the rate limit in the
// WHERE clause and the affected-row count it is read through — is in
// mysql_account_store_test.go; this file is about what the service decides.

type memoryAccountStore struct {
	account Account
	hash    string

	// allowNameChange stands in for the WHERE clause. The service cannot see it, which
	// is the point: it has to handle a refusal it did not predict.
	allowNameChange bool
	nameCalls       int
	lastBoundary    time.Time
	avatarURL       string
	err             error
}

func (store *memoryAccountStore) Account(_ context.Context, _ int64) (Account, error) {
	if store.err != nil {
		return Account{}, store.err
	}
	account := store.account
	account.HasPassword = store.hash != ""
	return account, nil
}

func (store *memoryAccountStore) UpdateName(
	_ context.Context, _ int64, name string, notChangedSince, changedAt time.Time,
) (bool, error) {
	store.nameCalls++
	store.lastBoundary = notChangedSince
	if store.err != nil {
		return false, store.err
	}
	if !store.allowNameChange {
		return false, nil
	}
	store.account.Name = name
	store.account.NameChangedAt = changedAt
	return true, nil
}

func (store *memoryAccountStore) UpdateAvatarURL(_ context.Context, _ int64, url string) error {
	if store.err != nil {
		return store.err
	}
	store.avatarURL = url
	store.account.AvatarURL = url
	return nil
}

func (store *memoryAccountStore) UpdatePasswordHash(_ context.Context, _ int64, hash string) error {
	if store.err != nil {
		return store.err
	}
	store.hash = hash
	return nil
}

type memoryAvatarStore struct {
	keys    []string
	types   []string
	sizes   []int
	putErr  error
	baseURL string
}

func (store *memoryAvatarStore) Put(
	_ context.Context, key string, body []byte, contentType string,
) (string, error) {
	store.keys = append(store.keys, key)
	store.types = append(store.types, contentType)
	store.sizes = append(store.sizes, len(body))
	if store.putErr != nil {
		return "", store.putErr
	}
	base := store.baseURL
	if base == "" {
		base = "https://file.2pick.test"
	}
	return base + "/" + key, nil
}

// accountHarness is a service wired around one account, whose user id is 7.
type accountHarness struct {
	service  *Service
	accounts *memoryAccountStore
	avatars  *memoryAvatarStore
	users    *fakeUsers
	sessions *memoryRefreshStore
}

const accountEmail = "holder@invalid.test"

func newAccountHarness(t *testing.T, passwordHash string) *accountHarness {
	t.Helper()

	accounts := &memoryAccountStore{
		account: Account{Name: "before", Email: accountEmail},
		hash:    passwordHash,
	}
	avatars := &memoryAvatarStore{}
	users := newFakeUsers()
	users.add(accountEmail, Credentials{UserID: 7, PasswordHash: passwordHash, Roles: []string{}})
	sessions := newMemoryRefreshStore()

	private, _, err := generateTestKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "2pick-go-api",
		PrivateKey: private, TTL: DefaultAccessTokenTTL,
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("load Asia/Taipei: %v", err)
	}
	service, err := NewService(ServiceOptions{
		Users:    users,
		Accounts: accounts,
		Avatars:  avatars,
		Sessions: sessions,
		Issuer:   issuer,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Timezone: taipei,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &accountHarness{
		service: service, accounts: accounts, avatars: avatars, users: users, sessions: sessions,
	}
}

// liveSessions counts the account's sessions that are neither revoked nor expired, which
// is what "still signed in on that device" means.
func (harness *accountHarness) liveSessions(userID int64) int {
	harness.sessions.mu.Lock()
	defer harness.sessions.mu.Unlock()
	live := 0
	for _, session := range harness.sessions.byHash {
		if session.UserID == userID && session.RevokedAt == nil {
			live++
		}
	}
	return live
}

func fieldCode(t *testing.T, err error, field string) string {
	t.Helper()
	var invalid *ErrAccountInvalid
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want an ErrAccountInvalid", err)
	}
	codes := invalid.Fields[field]
	if len(codes) != 1 {
		t.Fatalf("fields = %v, want exactly one code for %q", invalid.Fields, field)
	}
	return codes[0]
}

func TestChangeNameWritesAndReportsTheNewAccount(t *testing.T) {
	harness := newAccountHarness(t, "")
	harness.accounts.allowNameChange = true

	account, err := harness.service.ChangeName(context.Background(), 7, "  after  ")
	if err != nil {
		t.Fatalf("ChangeName() error = %v", err)
	}
	if account.Name != "after" {
		t.Errorf("Name = %q, want %q — the name is trimmed, as Laravel's validator did",
			account.Name, "after")
	}
	if harness.accounts.account.Name != "after" {
		t.Errorf("stored name = %q, want %q", harness.accounts.account.Name, "after")
	}
}

// The store's refusal is the rate limit. The service does not predict it — it reports it.
func TestChangeNameSurfacesTheStoresRefusalAsAFieldError(t *testing.T) {
	harness := newAccountHarness(t, "")
	harness.accounts.allowNameChange = false

	_, err := harness.service.ChangeName(context.Background(), 7, "after")
	if code := fieldCode(t, err, "name"); code != CodeNameChangeTooSoon {
		t.Errorf("code = %q, want %q", code, CodeNameChangeTooSoon)
	}
	if harness.accounts.account.Name != "before" {
		t.Errorf("stored name = %q; a refused change must not be written", harness.accounts.account.Name)
	}
}

// SUBMITTING THE NAME YOU ALREADY HAVE IS NOT A RATE-LIMIT REFUSAL. Laravel skipped the
// limit branch when the name matched, and the settings form posts the field whether or
// not it changed.
func TestChangeNameToTheSameNameNeitherWritesNorRefuses(t *testing.T) {
	harness := newAccountHarness(t, "")
	harness.accounts.account.Name = "unchanged"
	harness.accounts.allowNameChange = false

	account, err := harness.service.ChangeName(context.Background(), 7, "unchanged")
	if err != nil {
		t.Fatalf("ChangeName() error = %v; resubmitting the same name must succeed", err)
	}
	if account.Name != "unchanged" {
		t.Errorf("Name = %q, want %q", account.Name, "unchanged")
	}
	if harness.accounts.nameCalls != 0 {
		t.Errorf("the store was written %d times; an unchanged name must not reach it",
			harness.accounts.nameCalls)
	}
}

func TestChangeNameValidatesWhatLaravelValidated(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", CodeRequired},
		{"whitespace only", "   ", CodeRequired},
		{"too long", strings.Repeat("あ", MaxNameRunes+1), CodeTooLong},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newAccountHarness(t, "")
			harness.accounts.allowNameChange = true

			_, err := harness.service.ChangeName(context.Background(), 7, testCase.in)
			if code := fieldCode(t, err, "name"); code != testCase.want {
				t.Errorf("code = %q, want %q", code, testCase.want)
			}
			if harness.accounts.nameCalls != 0 {
				t.Error("an invalid name reached the store")
			}
		})
	}
}

// The limit counts calendar days in Asia/Taipei, not 24-hour windows: Laravel compared
// today() against name_updated_at->toDateString(). A change at 23:00 is therefore
// allowed again at 00:00, an hour later — and that is the rule, not a bug in the port.
func TestTheNameChangeBoundaryIsTaipeiMidnight(t *testing.T) {
	harness := newAccountHarness(t, "")
	harness.accounts.allowNameChange = true
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("load Asia/Taipei: %v", err)
	}
	// 00:30 Taipei, which is 16:30 the previous day in UTC. A boundary computed in UTC
	// would land on the wrong date and let a change through that was made "yesterday"
	// only in UTC terms.
	now := time.Date(2026, 8, 6, 0, 30, 0, 0, taipei)
	harness.service.now = func() time.Time { return now }

	if _, err := harness.service.ChangeName(context.Background(), 7, "after"); err != nil {
		t.Fatalf("ChangeName() error = %v", err)
	}

	wantBoundary := time.Date(2026, 8, 6, 0, 0, 0, 0, taipei)
	if !harness.accounts.lastBoundary.Equal(wantBoundary) {
		t.Errorf("boundary = %v, want %v (midnight in Asia/Taipei)",
			harness.accounts.lastBoundary, wantBoundary)
	}
}

func TestNameChangeAllowedAtIsTheDayAfterTheLastChange(t *testing.T) {
	harness := newAccountHarness(t, "")
	taipei, _ := time.LoadLocation("Asia/Taipei")

	if allowed := harness.service.NameChangeAllowedAt(Account{}); !allowed.IsZero() {
		t.Errorf("allowed = %v, want the zero time for an account that never changed its name", allowed)
	}

	changedAt := time.Date(2026, 8, 6, 23, 45, 0, 0, taipei)
	want := time.Date(2026, 8, 7, 0, 0, 0, 0, taipei)
	if allowed := harness.service.NameChangeAllowedAt(Account{NameChangedAt: changedAt}); !allowed.Equal(want) {
		t.Errorf("allowed = %v, want %v", allowed, want)
	}
}

func TestUploadAvatarStoresTheImageAndPointsTheAccountAtIt(t *testing.T) {
	harness := newAccountHarness(t, "")

	url, err := harness.service.UploadAvatar(context.Background(), 7, pngBytes(64),
		func(extension string) string { return "avatars/fixed." + extension })
	if err != nil {
		t.Fatalf("UploadAvatar() error = %v", err)
	}
	if url != "https://file.2pick.test/avatars/fixed.png" {
		t.Errorf("url = %q", url)
	}
	if harness.accounts.avatarURL != url {
		t.Errorf("stored url = %q, want %q", harness.accounts.avatarURL, url)
	}
	if len(harness.avatars.types) != 1 || harness.avatars.types[0] != "image/png" {
		t.Errorf("content types = %v, want one image/png", harness.avatars.types)
	}
}

// THE CHECK LARAVEL DECLARED AND NEVER RAN. Its rule was attached to a field named
// avatar_url while the file arrived as avatar, so `sometimes` skipped it: any file of any
// type was stored and served back under an image URL. The type comes from the bytes, not
// from the request, because the browser's Content-Type is the attacker's to choose.
func TestUploadAvatarRefusesWhatIsNotAnImage(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		want  string
	}{
		{"html", []byte("<html><body>hello</body></html>"), CodeUnsupportedImage},
		{"svg, which browsers execute script from", []byte(`<svg xmlns="http://www.w3.org/2000/svg">`), CodeUnsupportedImage},
		{"php", []byte("<?php system($_GET['c']); ?>"), CodeUnsupportedImage},
		{"empty", []byte{}, CodeRequired},
		{"a png header truncated below the magic", []byte("\x89PNG"), CodeUnsupportedImage},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newAccountHarness(t, "")

			_, err := harness.service.UploadAvatar(context.Background(), 7, testCase.bytes,
				func(extension string) string { return "avatars/x." + extension })
			if code := fieldCode(t, err, "avatar"); code != testCase.want {
				t.Errorf("code = %q, want %q", code, testCase.want)
			}
			if len(harness.avatars.keys) != 0 {
				t.Errorf("%d objects were stored; nothing must reach the bucket", len(harness.avatars.keys))
			}
			if harness.accounts.avatarURL != "" {
				t.Error("the account was pointed at a rejected upload")
			}
		})
	}
}

func TestUploadAvatarAcceptsTheFourFormatsTheSiteServes(t *testing.T) {
	cases := []struct {
		name      string
		bytes     []byte
		wantType  string
		extension string
	}{
		{"png", pngBytes(16), "image/png", "png"},
		{"jpeg", []byte("\xff\xd8\xff\xe0 the rest"), "image/jpeg", "jpg"},
		{"gif", []byte("GIF89a the rest"), "image/gif", "gif"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp", "webp"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newAccountHarness(t, "")

			if _, err := harness.service.UploadAvatar(context.Background(), 7, testCase.bytes,
				func(extension string) string { return "avatars/x." + extension }); err != nil {
				t.Fatalf("UploadAvatar() error = %v", err)
			}
			if harness.avatars.types[0] != testCase.wantType {
				t.Errorf("content type = %q, want %q", harness.avatars.types[0], testCase.wantType)
			}
			if want := "avatars/x." + testCase.extension; harness.avatars.keys[0] != want {
				t.Errorf("key = %q, want %q — the extension follows the sniffed format",
					harness.avatars.keys[0], want)
			}
		})
	}
}

func TestUploadAvatarRefusesAnImageOverTheLimit(t *testing.T) {
	harness := newAccountHarness(t, "")

	_, err := harness.service.UploadAvatar(context.Background(), 7, pngBytes(MaxAvatarBytes+1),
		func(extension string) string { return "avatars/x." + extension })
	if code := fieldCode(t, err, "avatar"); code != CodeTooLarge {
		t.Errorf("code = %q, want %q", code, CodeTooLarge)
	}
	if len(harness.avatars.keys) != 0 {
		t.Error("an oversized image reached the bucket")
	}
}

// The account is only pointed at the new URL once the object exists. The other order
// would leave a broken avatar on every storage failure.
func TestUploadAvatarLeavesTheAccountAloneWhenTheStoreFails(t *testing.T) {
	harness := newAccountHarness(t, "")
	harness.avatars.putErr = errors.New("bucket unreachable")

	if _, err := harness.service.UploadAvatar(context.Background(), 7, pngBytes(16),
		func(extension string) string { return "avatars/x." + extension }); err == nil {
		t.Fatal("UploadAvatar() returned no error although the bucket refused the write")
	}
	if harness.accounts.avatarURL != "" {
		t.Errorf("the account was pointed at %q after the upload failed", harness.accounts.avatarURL)
	}
}

func TestChangePasswordRotatesTheHashAndIssuesAFreshSession(t *testing.T) {
	current := hashFor(t, "the-old-password")
	harness := newAccountHarness(t, current)

	grant, err := harness.service.ChangePassword(context.Background(), 7,
		"the-old-password", "the-new-password", ClientInfo{IP: "203.0.113.9"})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if grant.Access.Token == "" || grant.Refresh.Token == "" {
		t.Fatal("no session was issued; the caller would be signed out by their own change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(harness.accounts.hash), []byte("the-new-password")); err != nil {
		t.Errorf("the stored hash does not verify against the new password: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(harness.accounts.hash), []byte("the-old-password")) == nil {
		t.Error("the old password still verifies")
	}
	// One live session: the one just issued. Everything older was revoked.
	if live := harness.liveSessions(7); live != 1 {
		t.Errorf("%d live sessions, want 1 — every session but the caller's must end", live)
	}
}

// The reason to change a password is that the old one may be known to someone else, and
// a refresh token issued while it was is what that someone is still holding.
func TestChangePasswordEndsTheAccountsOtherSessions(t *testing.T) {
	current := hashFor(t, "the-old-password")
	harness := newAccountHarness(t, current)
	ctx := context.Background()

	// Three devices signed in before the change.
	for device := 0; device < 3; device++ {
		if _, err := harness.service.Login(ctx, accountEmail, "the-old-password", ClientInfo{}); err != nil {
			t.Fatalf("seed a session: %v", err)
		}
	}
	if live := harness.liveSessions(7); live != 3 {
		t.Fatalf("%d live sessions before the change, want 3", live)
	}

	if _, err := harness.service.ChangePassword(ctx, 7,
		"the-old-password", "the-new-password", ClientInfo{}); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	if live := harness.liveSessions(7); live != 1 {
		t.Errorf("%d live sessions after the change, want 1", live)
	}
}

func TestChangePasswordRefusesAWrongCurrentPassword(t *testing.T) {
	harness := newAccountHarness(t, hashFor(t, "the-old-password"))

	_, err := harness.service.ChangePassword(context.Background(), 7,
		"not-the-old-password", "the-new-password", ClientInfo{})
	if code := fieldCode(t, err, "current_password"); code != CodeIncorrect {
		t.Errorf("code = %q, want %q", code, CodeIncorrect)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(harness.accounts.hash), []byte("the-old-password")); err != nil {
		t.Error("the password was changed despite a wrong current password")
	}
	if live := harness.liveSessions(7); live != 0 {
		t.Error("a failed change issued or revoked sessions")
	}
}

func TestChangePasswordValidatesTheNewPassword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", CodeRequired},
		{"one rune short", strings.Repeat("a", MinPasswordRunes-1), CodeTooShort},
		{"past what bcrypt reads", strings.Repeat("a", MaxPasswordBytes+1), CodeTooLong},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newAccountHarness(t, hashFor(t, "the-old-password"))

			_, err := harness.service.ChangePassword(context.Background(), 7,
				"the-old-password", testCase.in, ClientInfo{})
			if code := fieldCode(t, err, "new_password"); code != testCase.want {
				t.Errorf("code = %q, want %q", code, testCase.want)
			}
			if bcrypt.CompareHashAndPassword([]byte(harness.accounts.hash), []byte("the-old-password")) != nil {
				t.Error("the password was replaced although the new one was invalid")
			}
		})
	}
}

// An account with no password cannot change one: there is nothing to prove.
func TestChangePasswordRefusesAnAccountWithoutOne(t *testing.T) {
	harness := newAccountHarness(t, "")

	_, err := harness.service.ChangePassword(context.Background(), 7, "", "the-new-password", ClientInfo{})
	if code := fieldCode(t, err, "current_password"); code != CodeNoPasswordSet {
		t.Errorf("code = %q, want %q", code, CodeNoPasswordSet)
	}
}

// The path for the 11,040 accounts that signed in through Google and have an empty
// password column.
func TestSetInitialPasswordGivesAPasswordlessAccountOne(t *testing.T) {
	harness := newAccountHarness(t, "")

	grant, err := harness.service.SetInitialPassword(context.Background(), 7, "the-new-password", ClientInfo{})
	if err != nil {
		t.Fatalf("SetInitialPassword() error = %v", err)
	}
	if grant.Access.Token == "" {
		t.Error("no session was issued")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(harness.accounts.hash), []byte("the-new-password")); err != nil {
		t.Errorf("the stored hash does not verify: %v", err)
	}
	// Setting a first password revokes nothing: there was no password to have leaked,
	// so signing the account out of its other devices would cost something and buy
	// nothing. Only the session just issued is live because none existed before.
	if live := harness.liveSessions(7); live != 1 {
		t.Errorf("%d live sessions, want 1", live)
	}
}

// Without this, an access token — which does not prove knowledge of the password — would
// be enough to replace it, and a stolen token would become a stolen account.
func TestSetInitialPasswordRefusesAnAccountThatAlreadyHasOne(t *testing.T) {
	existing := hashFor(t, "the-old-password")
	harness := newAccountHarness(t, existing)

	_, err := harness.service.SetInitialPassword(context.Background(), 7, "the-new-password", ClientInfo{})
	if code := fieldCode(t, err, "new_password"); code != CodePasswordAlreadySet {
		t.Errorf("code = %q, want %q", code, CodePasswordAlreadySet)
	}
	if harness.accounts.hash != existing {
		t.Error("the existing password was replaced")
	}
}

func TestSetInitialPasswordValidatesTheNewPassword(t *testing.T) {
	harness := newAccountHarness(t, "")

	_, err := harness.service.SetInitialPassword(context.Background(), 7,
		strings.Repeat("a", MinPasswordRunes-1), ClientInfo{})
	if code := fieldCode(t, err, "new_password"); code != CodeTooShort {
		t.Errorf("code = %q, want %q", code, CodeTooShort)
	}
	if harness.accounts.hash != "" {
		t.Error("a too-short password was written")
	}
}

// A new password that verifies through the login path is the only proof that the write
// was the right shape: the hash is written by one query and read by another.
func TestAPasswordSetHereWorksForLogin(t *testing.T) {
	harness := newAccountHarness(t, "")

	if _, err := harness.service.SetInitialPassword(context.Background(), 7, "the-new-password", ClientInfo{}); err != nil {
		t.Fatalf("SetInitialPassword() error = %v", err)
	}
	// The login path reads through a different query, so the hash has to be handed to
	// it the way the database would: this is the only proof the write was usable.
	harness.users.add(accountEmail, Credentials{
		UserID: 7, PasswordHash: harness.accounts.hash, Roles: []string{}})

	if _, err := harness.service.Login(context.Background(),
		accountEmail, "the-new-password", ClientInfo{}); err != nil {
		t.Fatalf("Login() with the new password: %v", err)
	}
}

// Nil stores mean the endpoints answer "not configured" rather than panicking, which is
// how a process without the storage variables has to behave.
func TestAccountOperationsWithoutTheirStoresAreErrors(t *testing.T) {
	private, _, err := generateTestKeypair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	issuer, err := NewIssuer(IssuerConfig{
		Issuer: "http://localhost", Audience: "2pick-go-api",
		PrivateKey: private, TTL: DefaultAccessTokenTTL,
	})
	if err != nil {
		t.Fatalf("NewIssuer() error = %v", err)
	}
	service, err := NewService(ServiceOptions{
		Users:    newFakeUsers(),
		Sessions: newMemoryRefreshStore(),
		Issuer:   issuer,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx := context.Background()

	// ErrNotConfigured specifically, not just "an error": the API turns this one into a
	// 503 and everything else into a 500, and a client told 500 retries something that
	// will never work.
	operations := map[string]func() error{
		"Account":    func() error { _, err := service.Account(ctx, 7); return err },
		"ChangeName": func() error { _, err := service.ChangeName(ctx, 7, "after"); return err },
		"UploadAvatar": func() error {
			_, err := service.UploadAvatar(ctx, 7, pngBytes(16), func(string) string { return "k" })
			return err
		},
		"ChangePassword":     func() error { _, err := service.ChangePassword(ctx, 7, "a", "b", ClientInfo{}); return err },
		"SetInitialPassword": func() error { _, err := service.SetInitialPassword(ctx, 7, "b", ClientInfo{}); return err },
	}
	for name, operation := range operations {
		if err := operation(); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s() error = %v, want ErrNotConfigured", name, err)
		}
	}
}

func hashFor(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash %q: %v", password, err)
	}
	return string(hash)
}

// pngBytes is a valid PNG signature followed by filler, which is all the sniffer reads.
func pngBytes(size int) []byte {
	image := []byte("\x89PNG\r\n\x1a\n")
	if size <= len(image) {
		return image
	}
	return append(image, make([]byte, size-len(image))...)
}
