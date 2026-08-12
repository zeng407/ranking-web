package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// memoryUserWriter stands in for MySQLUserStore's write half, including the unique index
// that arbitrates two sign-ups for one address.
type memoryUserWriter struct {
	mutex   sync.Mutex
	emails  map[string]int64
	created []NewUser
	nextID  int64
	// existsErr makes the pre-check fail, to prove the error is surfaced rather than
	// swallowed into "address is free".
	existsErr error
	// createErr fails the insert once, without the address ever appearing to the
	// pre-check. That is exactly the shape of two sign-ups racing: both checks pass and
	// the unique index refuses the loser.
	createErr error
}

func newMemoryUserWriter() *memoryUserWriter {
	return &memoryUserWriter{emails: map[string]int64{}, nextID: 500}
}

func (writer *memoryUserWriter) EmailExists(_ context.Context, email string) (bool, error) {
	if writer.existsErr != nil {
		return false, writer.existsErr
	}
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	_, found := writer.emails[strings.ToLower(strings.TrimSpace(email))]
	return found, nil
}

func (writer *memoryUserWriter) CreateUser(_ context.Context, record NewUser) (Credentials, error) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	if writer.createErr != nil {
		failure := writer.createErr
		writer.createErr = nil
		return Credentials{}, failure
	}
	key := strings.ToLower(record.Email)
	if _, taken := writer.emails[key]; taken {
		return Credentials{}, ErrOAuthEmailTaken
	}
	writer.nextID++
	writer.emails[key] = writer.nextID
	writer.created = append(writer.created, record)
	return Credentials{UserID: writer.nextID, PasswordHash: record.PasswordHash, Roles: []string{}}, nil
}

// registerHarness wires the session service with a writable user store.
func newRegisterHarness(t *testing.T) (*Service, *memoryUserWriter) {
	t.Helper()
	harness := newAuthHarness(t)
	writer := newMemoryUserWriter()
	harness.service.registrations = writer
	return harness.service, writer
}

func validRegistration() Registration {
	return Registration{
		Name:                 "New Player",
		Email:                "new@example.test",
		Password:             "a-good-password",
		PasswordConfirmation: "a-good-password",
	}
}

func TestRegisterCreatesAnAccountAndSignsItIn(t *testing.T) {
	service, writer := newRegisterHarness(t)

	grant, err := service.Register(context.Background(), validRegistration(),
		ClientInfo{IP: "203.0.113.7", UserAgent: "probe/1.0"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Signed in as part of registering, matching Laravel's RegistersUsers trait.
	if grant.Access.Token == "" || grant.Refresh.Token == "" {
		t.Error("registration did not issue a session")
	}
	if grant.UserID == 0 {
		t.Error("no user id was returned")
	}

	if len(writer.created) != 1 {
		t.Fatalf("%d accounts created, want 1", len(writer.created))
	}
	created := writer.created[0]

	// THE PASSWORD MUST BE HASHED, AND AT THE COST LARAVEL USES. A cheaper cost still
	// verifies, so nothing would fail — every new account would just be easier to crack
	// than every old one.
	if created.PasswordHash == validRegistration().Password {
		t.Fatal("the password was stored in clear")
	}
	cost, err := bcrypt.Cost([]byte(created.PasswordHash))
	if err != nil {
		t.Fatalf("the stored value is not a bcrypt hash: %v", err)
	}
	if cost != BcryptCost {
		t.Errorf("bcrypt cost = %d, want %d (config/hashing.php rounds)", cost, BcryptCost)
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(created.PasswordHash), []byte(validRegistration().Password)); err != nil {
		t.Errorf("the stored hash does not verify against the password: %v", err)
	}
}

// The account created here must be able to log in afterwards. Registration and login are
// separate code paths over the same column, and a mismatch between them would only show
// up on the user's second visit.
func TestARegisteredAccountCanLogIn(t *testing.T) {
	service, writer := newRegisterHarness(t)
	registration := validRegistration()

	if _, err := service.Register(context.Background(), registration, ClientInfo{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// The login path reads through UserStore, which the harness backs with fakeUsers,
	// so the new account is copied across by hand — the point being tested is that the
	// hash written by one path is accepted by the other.
	created := writer.created[0]
	users, ok := service.users.(*fakeUsers)
	if !ok {
		t.Fatalf("unexpected user store %T", service.users)
	}
	users.add(created.Email, Credentials{UserID: 501, PasswordHash: created.PasswordHash, Roles: []string{}})

	if _, err := service.Login(context.Background(), created.Email, registration.Password, ClientInfo{}); err != nil {
		t.Fatalf("a freshly registered account could not log in: %v", err)
	}
	if _, err := service.Login(context.Background(), created.Email, "the-wrong-one", ClientInfo{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("the wrong password was accepted: %v", err)
	}
}

// The rules mirror config/setting.php. Each case is one field the form displays, so a
// drift here shows up as a form that either rejects what Laravel accepted or accepts
// what the column cannot hold.
func TestRegisterValidatesTheSameRulesLaravelDid(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Registration)
		field    string
		wantCode string
	}{
		{"no name", func(r *Registration) { r.Name = "" }, "name", CodeRequired},
		{"whitespace name", func(r *Registration) { r.Name = "   " }, "name", CodeRequired},
		{
			"name over twenty runes",
			func(r *Registration) { r.Name = strings.Repeat("排", MaxNameRunes+1) },
			"name", CodeTooLong,
		},
		{"no email", func(r *Registration) { r.Email = "" }, "email", CodeRequired},
		{
			"email over fifty characters",
			func(r *Registration) { r.Email = strings.Repeat("a", 45) + "@example.test" },
			"email", CodeTooLong,
		},
		{"email with no at", func(r *Registration) { r.Email = "not-an-address" }, "email", CodeInvalidEmail},
		{"email with two ats", func(r *Registration) { r.Email = "a@b@example.test" }, "email", CodeInvalidEmail},
		{"email with no host dot", func(r *Registration) { r.Email = "someone@localhost" }, "email", CodeInvalidEmail},
		{"email with a space", func(r *Registration) { r.Email = "some one@example.test" }, "email", CodeInvalidEmail},
		{"no password", func(r *Registration) { r.Password = ""; r.PasswordConfirmation = "" }, "password", CodeRequired},
		{
			"password under eight",
			func(r *Registration) { r.Password = "short1"; r.PasswordConfirmation = "short1" },
			"password", CodeTooShort,
		},
		{
			// bcrypt truncates at 72 bytes, so a longer password is not more secure and
			// the stored hash would not cover what was typed.
			"password over seventy-two bytes",
			func(r *Registration) {
				long := strings.Repeat("x", MaxPasswordBytes+1)
				r.Password, r.PasswordConfirmation = long, long
			},
			"password", CodeTooLong,
		},
		{
			"confirmation does not match",
			func(r *Registration) { r.PasswordConfirmation = "something-else" },
			"password_confirmation", CodeMismatch,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, writer := newRegisterHarness(t)
			registration := validRegistration()
			testCase.mutate(&registration)

			_, err := service.Register(context.Background(), registration, ClientInfo{})

			var invalid *ErrRegistrationInvalid
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %v, want ErrRegistrationInvalid", err)
			}
			codes := invalid.Fields[testCase.field]
			if len(codes) == 0 || codes[0] != testCase.wantCode {
				t.Errorf("%s = %v, want [%s]", testCase.field, codes, testCase.wantCode)
			}
			if len(writer.created) != 0 {
				t.Error("an account was created despite the refusal")
			}
		})
	}
}

// A name of exactly the limit is accepted: an off-by-one here would reject the longest
// name the old form allowed.
func TestRegisterAcceptsTheExactLimits(t *testing.T) {
	service, _ := newRegisterHarness(t)
	registration := validRegistration()
	registration.Name = strings.Repeat("排", MaxNameRunes)
	registration.Password = strings.Repeat("x", MinPasswordRunes)
	registration.PasswordConfirmation = registration.Password

	if _, err := service.Register(context.Background(), registration, ClientInfo{}); err != nil {
		t.Fatalf("a registration at the exact limits was refused: %v", err)
	}
}

func TestRegisterTrimsNameAndAddress(t *testing.T) {
	service, writer := newRegisterHarness(t)
	registration := validRegistration()
	registration.Name = "  Padded Name  "
	registration.Email = "  padded@example.test  "

	if _, err := service.Register(context.Background(), registration, ClientInfo{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	created := writer.created[0]
	if created.Name != "Padded Name" {
		t.Errorf("name = %q, want it trimmed", created.Name)
	}
	if created.Email != "padded@example.test" {
		t.Errorf("email = %q, want it trimmed", created.Email)
	}
}

// The password is NOT trimmed. A trailing space is a legitimate character, and stripping
// it would mean the account can never be logged into with the password the user typed.
func TestRegisterDoesNotTrimThePassword(t *testing.T) {
	service, writer := newRegisterHarness(t)
	registration := validRegistration()
	registration.Password = " padded password "
	registration.PasswordConfirmation = registration.Password

	if _, err := service.Register(context.Background(), registration, ClientInfo{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(writer.created[0].PasswordHash), []byte(" padded password ")); err != nil {
		t.Errorf("the password was altered before hashing: %v", err)
	}
}

func TestRegisterRefusesATakenAddress(t *testing.T) {
	service, writer := newRegisterHarness(t)
	writer.emails["new@example.test"] = 1

	_, err := service.Register(context.Background(), validRegistration(), ClientInfo{})
	var invalid *ErrRegistrationInvalid
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want ErrRegistrationInvalid", err)
	}
	if codes := invalid.Fields["email"]; len(codes) == 0 || codes[0] != CodeTaken {
		t.Errorf("email = %v, want [%s]", codes, CodeTaken)
	}
}

// Two sign-ups for the same address at once: both pre-checks pass and the unique index
// refuses the loser. It has to become a field error, not a 500 — the user needs to be
// told the address is taken, not that something broke.
func TestASimultaneousSignUpBecomesAFieldError(t *testing.T) {
	service, writer := newRegisterHarness(t)
	// The address is invisible to the check and rejected by the insert, which is what
	// the loser of the race sees.
	writer.createErr = ErrOAuthEmailTaken

	_, err := service.Register(context.Background(), validRegistration(), ClientInfo{})

	var invalid *ErrRegistrationInvalid
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want ErrRegistrationInvalid rather than a raw failure", err)
	}
	if codes := invalid.Fields["email"]; len(codes) == 0 || codes[0] != CodeTaken {
		t.Errorf("email = %v, want [%s]", codes, CodeTaken)
	}
}

// A store failure on the address check must surface, not be read as "the address is
// free" — which would let the insert be attempted against an unknown state.
func TestRegisterSurfacesAStoreFailure(t *testing.T) {
	service, writer := newRegisterHarness(t)
	failure := errors.New("the database is down")
	writer.existsErr = failure

	_, err := service.Register(context.Background(), validRegistration(), ClientInfo{})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the store failure", err)
	}
	var invalid *ErrRegistrationInvalid
	if errors.As(err, &invalid) {
		t.Error("a store failure was reported as a validation error")
	}
}

// Without a writer configured, Register refuses rather than panicking on a nil store.
func TestRegisterWithoutAWriterIsAnError(t *testing.T) {
	harness := newAuthHarness(t)
	if _, err := harness.service.Register(context.Background(), validRegistration(), ClientInfo{}); err == nil {
		t.Fatal("Register() succeeded with no writer configured")
	}
}

func TestLooksLikeEmailAcceptsRealAddresses(t *testing.T) {
	// Deliberately permissive: a stricter pattern rejects addresses that exist, and
	// these are shapes real accounts use.
	for _, address := range []string{
		"a@b.co",
		"first.last@example.co.uk",
		"user+tag@example.test",
		"user_name@sub.domain.example",
		"123@456.test",
	} {
		if !looksLikeEmail(address) {
			t.Errorf("%q was rejected", address)
		}
	}
	for _, address := range []string{
		"", "@example.test", "user@", "user@host", "user@.test", "user@test.", "a b@example.test",
	} {
		if looksLikeEmail(address) {
			t.Errorf("%q was accepted", address)
		}
	}
}
