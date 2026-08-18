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

	"2pick.app/backend/internal/mailer"
)

// The forgot-password rules, against in-memory stores. The MySQL half — the claim that
// makes a link single-use — is in mysql_password_reset_store_test.go; this file is about
// what the service decides, and above all about what it refuses to tell the caller.

// memoryResetStore reproduces the MySQL store's semantics, including the conditional
// claim. Anything looser would let these tests pass while a link stayed reusable.
type memoryResetStore struct {
	mu      sync.Mutex
	rows    []*storedReset
	writeEr error
	readErr error
}

type storedReset struct {
	userID    int64
	tokenHash string
	requested time.Time
	expires   time.Time
	usedAt    *time.Time
}

func (store *memoryResetStore) Create(_ context.Context, record NewPasswordReset) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.writeEr != nil {
		return store.writeEr
	}
	store.rows = append(store.rows, &storedReset{
		userID: record.UserID, tokenHash: record.TokenHash,
		requested: record.RequestedAt, expires: record.ExpiresAt,
	})
	return nil
}

func (store *memoryResetStore) LastRequestedAt(_ context.Context, userID int64) (time.Time, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.readErr != nil {
		return time.Time{}, false, store.readErr
	}
	var newest time.Time
	found := false
	for _, row := range store.rows {
		if row.userID == userID && row.requested.After(newest) {
			newest, found = row.requested, true
		}
	}
	return newest, found, nil
}

func (store *memoryResetStore) Consume(_ context.Context, tokenHash string, at time.Time) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, row := range store.rows {
		if row.tokenHash != tokenHash {
			continue
		}
		// The AND used_at IS NULL AND expires_at > ? of the real statement.
		if row.usedAt != nil || !row.expires.After(at) {
			return 0, ErrResetTokenInvalid
		}
		stamp := at
		row.usedAt = &stamp
		return row.userID, nil
	}
	return 0, ErrResetTokenInvalid
}

func (store *memoryResetStore) count() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.rows)
}

type memoryMailer struct {
	mu       sync.Mutex
	messages []mailer.Message
	sendErr  error
}

func (sender *memoryMailer) Send(_ context.Context, message mailer.Message) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if sender.sendErr != nil {
		return sender.sendErr
	}
	sender.messages = append(sender.messages, message)
	return nil
}

func (sender *memoryMailer) count() int {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return len(sender.messages)
}

func (sender *memoryMailer) last() mailer.Message {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) == 0 {
		return mailer.Message{}
	}
	return sender.messages[len(sender.messages)-1]
}

type denyingLimiter struct {
	allow bool
	err   error
	calls int
}

func (limiter *denyingLimiter) AllowReset(_ context.Context, _ string) (bool, error) {
	limiter.calls++
	return limiter.allow, limiter.err
}

const resetEmail = "holder@invalid.test"

type resetHarness struct {
	service  *Service
	accounts *memoryAccountStore
	resets   *memoryResetStore
	mail     *memoryMailer
	sessions *memoryRefreshStore
	now      time.Time
}

// newResetHarness wires a service around one account, whose user id is 7.
func newResetHarness(t *testing.T) *resetHarness {
	t.Helper()
	return newResetHarnessWith(t, func(*ServiceOptions) {})
}

func newResetHarnessWith(t *testing.T, adjust func(*ServiceOptions)) *resetHarness {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("the-old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	users := newFakeUsers()
	users.add(resetEmail, Credentials{UserID: 7, PasswordHash: string(hash), Roles: []string{}})

	harness := &resetHarness{
		accounts: &memoryAccountStore{account: Account{Name: "holder", Email: resetEmail}, hash: string(hash)},
		resets:   &memoryResetStore{},
		mail:     &memoryMailer{},
		sessions: newMemoryRefreshStore(),
		now:      time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC),
	}

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

	options := ServiceOptions{
		Users:    users,
		Accounts: harness.accounts,
		Sessions: harness.sessions,
		Resets:   harness.resets,
		Mail:     harness.mail,
		// The trailing slash is deliberate: APP_URL carries one often enough that a link
		// built by concatenation would have two.
		AppURL: "https://2pick.test/",
		Issuer: issuer,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return harness.now },
	}
	adjust(&options)

	service, err := NewService(options)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	harness.service = service
	return harness
}

// tokenFromMail pulls the token out of the mailed link, which is the only place it exists
// — the store holds its hash, and the service never returns it.
func tokenFromMail(t *testing.T, message mailer.Message) string {
	t.Helper()
	_, after, found := strings.Cut(message.TextBody, "/password/reset/")
	if !found {
		t.Fatalf("the mail carries no reset link:\n%s", message.TextBody)
	}
	token, _, _ := strings.Cut(after, "\r\n")
	if strings.TrimSpace(token) == "" {
		t.Fatalf("the link carries no token:\n%s", message.TextBody)
	}
	return token
}

func (harness *resetHarness) request(t *testing.T, email, locale string) {
	t.Helper()
	if err := harness.service.RequestPasswordReset(
		context.Background(), email, locale, ClientInfo{IP: "203.0.113.7"}); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v", err)
	}
}

func (harness *resetHarness) liveSessions(userID int64) int {
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

func TestRequestPasswordResetMailsALinkTheResetAccepts(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, resetEmail, "zh_TW")

	if harness.mail.count() != 1 {
		t.Fatalf("mails sent = %d, want 1", harness.mail.count())
	}
	message := harness.mail.last()
	if message.To != resetEmail {
		t.Errorf("recipient = %q, want %q", message.To, resetEmail)
	}
	if !strings.Contains(message.TextBody, "https://2pick.test/zh-tw/password/reset/") {
		t.Errorf("the link is not the SPA's own route:\n%s", message.TextBody)
	}
	if strings.Contains(message.TextBody, "2pick.test//") {
		t.Errorf("APP_URL's trailing slash was not trimmed:\n%s", message.TextBody)
	}

	grant, err := harness.service.ResetPassword(
		context.Background(), tokenFromMail(t, message), "a-brand-new-password", ClientInfo{})
	if err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if grant.UserID != 7 {
		t.Errorf("user id = %d, want 7", grant.UserID)
	}
	if grant.Access.Token == "" || grant.Refresh.Token == "" {
		t.Error("a finished reset did not issue a session")
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(harness.accounts.hash), []byte("a-brand-new-password")); err != nil {
		t.Errorf("the stored hash does not match the new password: %v", err)
	}
}

// THE ANSWER FOR AN ADDRESS WITH NO ACCOUNT IS THE ANSWER FOR ONE WITH AN ACCOUNT. Any
// difference — an error, a different status, a measurably different response — turns this
// form into a way to test whether an address has an account here.
func TestRequestPasswordResetSaysNothingAboutAnUnknownAddress(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, "nobody@invalid.test", "zh_TW")

	if harness.mail.count() != 0 {
		t.Errorf("mails sent = %d, want 0", harness.mail.count())
	}
	if harness.resets.count() != 0 {
		t.Errorf("rows written = %d, want 0", harness.resets.count())
	}
}

func TestRequestPasswordResetMailsAtMostOnceAMinute(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, resetEmail, "zh_TW")

	harness.now = harness.now.Add(ResetThrottle - time.Second)
	harness.request(t, resetEmail, "zh_TW")
	if harness.mail.count() != 1 {
		t.Fatalf("mails sent = %d within the throttle, want 1", harness.mail.count())
	}
	if harness.resets.count() != 1 {
		t.Errorf("rows written = %d within the throttle, want 1", harness.resets.count())
	}

	harness.now = harness.now.Add(2 * time.Second)
	harness.request(t, resetEmail, "zh_TW")
	if harness.mail.count() != 2 {
		t.Errorf("mails sent = %d after the throttle, want 2", harness.mail.count())
	}
}

func TestRequestPasswordResetReportsAMalformedAddress(t *testing.T) {
	harness := newResetHarness(t)

	for _, testCase := range []struct{ email, code string }{
		{"", CodeRequired},
		{"   ", CodeRequired},
		{"not-an-address", CodeInvalidEmail},
		{strings.Repeat("a", MaxEmailBytes) + "@invalid.test", CodeTooLong},
	} {
		err := harness.service.RequestPasswordReset(
			context.Background(), testCase.email, "zh_TW", ClientInfo{})
		if got := fieldCode(t, err, "email"); got != testCase.code {
			t.Errorf("%q gave %q, want %q", testCase.email, got, testCase.code)
		}
	}
	if harness.mail.count() != 0 {
		t.Errorf("mails sent = %d, want 0", harness.mail.count())
	}
}

func TestRequestPasswordResetIsNotConfiguredWithoutASender(t *testing.T) {
	harness := newResetHarnessWith(t, func(options *ServiceOptions) { options.Mail = nil })

	err := harness.service.RequestPasswordReset(context.Background(), resetEmail, "zh_TW", ClientInfo{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func TestRequestPasswordResetIsNotConfiguredWithoutASiteToLinkTo(t *testing.T) {
	harness := newResetHarnessWith(t, func(options *ServiceOptions) { options.AppURL = "" })

	err := harness.service.RequestPasswordReset(context.Background(), resetEmail, "zh_TW", ClientInfo{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

// The per-account throttle cannot see a script working through a list of addresses, so a
// refusal here has to stop the mail — while still telling the caller nothing.
func TestRequestPasswordResetStopsAtThePerSourceCap(t *testing.T) {
	limiter := &denyingLimiter{allow: false}
	harness := newResetHarnessWith(t, func(options *ServiceOptions) { options.ResetLimiter = limiter })

	harness.request(t, resetEmail, "zh_TW")
	if limiter.calls != 1 {
		t.Errorf("limiter calls = %d, want 1", limiter.calls)
	}
	if harness.mail.count() != 0 {
		t.Errorf("mails sent = %d, want 0", harness.mail.count())
	}
}

// A Redis outage must not take password recovery down with it: the per-account throttle
// is still in force, and the alternative is a site nobody can get back into.
func TestRequestPasswordResetSurvivesALimiterFailure(t *testing.T) {
	limiter := &denyingLimiter{err: errors.New("redis is down")}
	harness := newResetHarnessWith(t, func(options *ServiceOptions) { options.ResetLimiter = limiter })

	harness.request(t, resetEmail, "zh_TW")
	if harness.mail.count() != 1 {
		t.Errorf("mails sent = %d, want 1", harness.mail.count())
	}
}

// A relay that refuses the message must not be reported to the caller either: "we could
// not mail you" only happens for an address that has an account.
func TestRequestPasswordResetHidesAMailFailure(t *testing.T) {
	harness := newResetHarness(t)
	harness.mail.sendErr = errors.New("the relay said no")

	if err := harness.service.RequestPasswordReset(
		context.Background(), resetEmail, "zh_TW", ClientInfo{}); err != nil {
		t.Fatalf("RequestPasswordReset() error = %v", err)
	}
}

func TestResetPasswordRefusesASecondUseOfTheSameLink(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, resetEmail, "zh_TW")
	token := tokenFromMail(t, harness.mail.last())

	if _, err := harness.service.ResetPassword(
		context.Background(), token, "the-first-new-password", ClientInfo{}); err != nil {
		t.Fatalf("the first reset failed: %v", err)
	}

	_, err := harness.service.ResetPassword(
		context.Background(), token, "the-second-new-password", ClientInfo{})
	if got := fieldCode(t, err, "token"); got != CodeInvalid {
		t.Errorf("code = %q, want %q", got, CodeInvalid)
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(harness.accounts.hash), []byte("the-first-new-password")); err != nil {
		t.Error("the second use overwrote the password set by the first")
	}
}

func TestResetPasswordRefusesAnExpiredLink(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, resetEmail, "zh_TW")
	token := tokenFromMail(t, harness.mail.last())

	harness.now = harness.now.Add(ResetTokenTTL + time.Second)
	_, err := harness.service.ResetPassword(
		context.Background(), token, "a-brand-new-password", ClientInfo{})
	if got := fieldCode(t, err, "token"); got != CodeInvalid {
		t.Errorf("code = %q, want %q", got, CodeInvalid)
	}
}

// EXPIRED, ALREADY USED AND NEVER ISSUED ALL REPORT THE SAME CODE. Telling them apart
// would confirm to whoever is submitting guesses that a token was real.
func TestResetPasswordRefusesATokenItNeverIssued(t *testing.T) {
	harness := newResetHarness(t)

	_, err := harness.service.ResetPassword(
		context.Background(), "a-token-nobody-issued", "a-brand-new-password", ClientInfo{})
	if got := fieldCode(t, err, "token"); got != CodeInvalid {
		t.Errorf("code = %q, want %q", got, CodeInvalid)
	}
}

func TestResetPasswordRequiresAToken(t *testing.T) {
	harness := newResetHarness(t)

	_, err := harness.service.ResetPassword(
		context.Background(), "  ", "a-brand-new-password", ClientInfo{})
	if got := fieldCode(t, err, "token"); got != CodeRequired {
		t.Errorf("code = %q, want %q", got, CodeRequired)
	}
}

// A password the rules refuse must not spend the link, or a typo would cost the user
// another mail and another minute of waiting.
func TestResetPasswordKeepsTheLinkWhenThePasswordIsRefused(t *testing.T) {
	harness := newResetHarness(t)
	harness.request(t, resetEmail, "zh_TW")
	token := tokenFromMail(t, harness.mail.last())

	_, err := harness.service.ResetPassword(context.Background(), token, "short", ClientInfo{})
	if got := fieldCode(t, err, "new_password"); got != CodeTooShort {
		t.Fatalf("code = %q, want %q", got, CodeTooShort)
	}

	if _, err := harness.service.ResetPassword(
		context.Background(), token, "a-brand-new-password", ClientInfo{}); err != nil {
		t.Fatalf("the link no longer works after a refused password: %v", err)
	}
}

// A reset is what someone does when they think the password leaked, so the sessions that
// password opened have to end — the same rule ChangePassword follows.
func TestResetPasswordEndsTheAccountsOtherSessions(t *testing.T) {
	harness := newResetHarness(t)
	if _, err := harness.service.Login(
		context.Background(), resetEmail, "the-old-password", ClientInfo{}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if harness.liveSessions(7) != 1 {
		t.Fatalf("live sessions before = %d, want 1", harness.liveSessions(7))
	}

	harness.request(t, resetEmail, "zh_TW")
	grant, err := harness.service.ResetPassword(
		context.Background(), tokenFromMail(t, harness.mail.last()), "a-brand-new-password", ClientInfo{})
	if err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}

	// One live session: the one this reset just issued.
	if live := harness.liveSessions(7); live != 1 {
		t.Errorf("live sessions after = %d, want 1", live)
	}
	if grant.Refresh.Token == "" {
		t.Error("the reset did not issue a session of its own")
	}
}

func TestPasswordResetLinkCarriesTheLocalesOwnPrefix(t *testing.T) {
	for locale, prefix := range map[string]string{
		"zh_TW":     "zh-tw",
		"en":        "en",
		"ja":        "ja",
		"":          "zh-tw",
		"pt_BR":     "zh-tw",
		"not-a-tag": "zh-tw",
	} {
		message := passwordResetMessage(locale, resetEmail, "https://2pick.test", "the-token")
		want := "https://2pick.test/" + prefix + "/password/reset/the-token"
		if !strings.Contains(message.TextBody, want) {
			t.Errorf("locale %q gave:\n%s\nwant a link to %s", locale, message.TextBody, want)
		}
		if message.Subject == "" {
			t.Errorf("locale %q has no subject", locale)
		}
	}
}
