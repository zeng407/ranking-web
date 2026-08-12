package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	testGoogleClientID     = "1234567890-abcdefg.apps.googleusercontent.com"
	testGoogleClientSecret = "GOCSPX-test-secret"
	testGoogleRedirectURL  = "http://localhost:8080/api/v1/auth/oauth/google/callback"
)

// unsignedIDToken builds a JWT-shaped token with the given claims and a signature that
// is not checked. That is the point: see claimsFromIDToken for why the signature is not
// what establishes trust on this path, and note that these tests would pass with a real
// signature too — nothing here depends on it being absent.
func unsignedIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("encode header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("encode claims: %v", err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	return encode(header) + "." + encode(payload) + "." + encode([]byte("not-a-real-signature"))
}

func validGoogleClaims(now time.Time) map[string]any {
	return map[string]any{
		"iss":            "https://accounts.google.com",
		"aud":            testGoogleClientID,
		"sub":            "104567890123456789012",
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"email":          "player@example.test",
		"email_verified": true,
		"name":           "Player One",
		"picture":        "https://lh3.googleusercontent.test/a/photo",
	}
}

// googleStub is a stand-in token endpoint that records the form it was posted.
type googleStub struct {
	server     *httptest.Server
	lastForm   url.Values
	status     int
	body       string
	calls      int
	rawRequest string
}

func newGoogleStub(t *testing.T, idToken string) *googleStub {
	t.Helper()
	stub := &googleStub{status: http.StatusOK}
	stub.body = `{"access_token":"ya29.a0-test","expires_in":3599,"token_type":"Bearer",` +
		`"id_token":"` + idToken + `"}`

	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls++
		if err := r.ParseForm(); err != nil {
			t.Errorf("the token request was not a valid form: %v", err)
		}
		stub.lastForm = r.PostForm
		stub.rawRequest = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.body))
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func newTestGoogleProvider(t *testing.T, tokenEndpoint string, now time.Time) *GoogleProvider {
	t.Helper()
	provider, err := NewGoogleProvider(GoogleConfig{
		ClientID:      testGoogleClientID,
		ClientSecret:  testGoogleClientSecret,
		RedirectURL:   testGoogleRedirectURL,
		TokenEndpoint: tokenEndpoint,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider() error = %v", err)
	}
	return provider
}

func TestGoogleAuthorizationURLCarriesEverythingGoogleNeeds(t *testing.T) {
	provider := newTestGoogleProvider(t, "", time.Now())
	raw := provider.AuthorizationURL("the-state", "the-challenge")

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("the authorization url does not parse: %v", err)
	}
	if parsed.Host != "accounts.google.com" {
		t.Errorf("host = %q", parsed.Host)
	}

	query := parsed.Query()
	expected := map[string]string{
		"client_id":             testGoogleClientID,
		"redirect_uri":          testGoogleRedirectURL,
		"response_type":         "code",
		"state":                 "the-state",
		"code_challenge":        "the-challenge",
		"code_challenge_method": "S256",
	}
	for key, want := range expected {
		if got := query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// openid is what makes Google return an id_token at all; without it the exchange
	// succeeds and then has no identity in it.
	scopes := strings.Fields(query.Get("scope"))
	for _, required := range []string{"openid", "email", "profile"} {
		if !containsString(scopes, required) {
			t.Errorf("scope %q is missing from %v", required, scopes)
		}
	}
	// No offline access: a refresh token would let this server act as the user later,
	// which nothing does.
	if query.Get("access_type") == "offline" {
		t.Error("offline access is requested but no refresh token is stored or used")
	}
}

func TestGoogleExchangeSendsTheSecretAndTheVerifier(t *testing.T) {
	now := time.Now()
	stub := newGoogleStub(t, unsignedIDToken(t, validGoogleClaims(now)))
	provider := newTestGoogleProvider(t, stub.server.URL, now)

	identity, err := provider.Exchange(context.Background(), "the-code", "the-verifier")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	expected := map[string]string{
		"code":          "the-code",
		"client_id":     testGoogleClientID,
		"client_secret": testGoogleClientSecret,
		"redirect_uri":  testGoogleRedirectURL,
		"grant_type":    "authorization_code",
		// Without this the PKCE challenge sent at the start is never proven, and the
		// whole exercise is decorative.
		"code_verifier": "the-verifier",
	}
	for key, want := range expected {
		if got := stub.lastForm.Get(key); got != want {
			t.Errorf("form %s = %q, want %q", key, got, want)
		}
	}

	if identity.Subject != "104567890123456789012" {
		t.Errorf("subject = %q", identity.Subject)
	}
	if identity.Email != "player@example.test" {
		t.Errorf("email = %q", identity.Email)
	}
	if !identity.EmailVerified {
		t.Error("email_verified was true in the token but false in the identity")
	}
	if identity.Name != "Player One" {
		t.Errorf("name = %q", identity.Name)
	}
	if identity.AvatarURL == "" {
		t.Error("no avatar url was read")
	}
}

// The identity comes out of the id_token, not out of the access token. An exchange that
// returns no id_token has to fail rather than produce an empty identity — which would
// otherwise be a login as nobody.
func TestGoogleExchangeRejectsAResponseWithNoIDToken(t *testing.T) {
	stub := newGoogleStub(t, "")
	stub.body = `{"access_token":"ya29.a0-test","token_type":"Bearer"}`
	provider := newTestGoogleProvider(t, stub.server.URL, time.Now())

	if _, err := provider.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("Exchange() succeeded with no id_token in the response")
	}
}

// A 400 with an OAuth error body is the shape Google uses for a replayed or expired
// code. Treating it as a success would sign someone in on a failed exchange.
func TestGoogleExchangeReportsAnErrorResponse(t *testing.T) {
	stub := newGoogleStub(t, "")
	stub.status = http.StatusBadRequest
	stub.body = `{"error":"invalid_grant","error_description":"Bad Request"}`
	provider := newTestGoogleProvider(t, stub.server.URL, time.Now())

	_, err := provider.Exchange(context.Background(), "code", "verifier")
	if err == nil {
		t.Fatal("Exchange() succeeded on a 400")
	}
	// The provider's wording belongs in a log, and the caller wraps this in
	// ErrOAuthProviderFailed rather than showing it.
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("the error does not say what Google said: %v", err)
	}
}

// A 200 that carries an error field. Some endpoints do this, and keying only on the
// status would accept it.
func TestGoogleExchangeReportsAnErrorFieldInATwoHundred(t *testing.T) {
	stub := newGoogleStub(t, "")
	stub.body = `{"error":"invalid_client"}`
	provider := newTestGoogleProvider(t, stub.server.URL, time.Now())

	if _, err := provider.Exchange(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("Exchange() succeeded on a 200 carrying an error")
	}
}

// THE CLAIMS THAT ARE STILL CHECKED. The signature is not what establishes trust here
// — the TLS connection to the token endpoint is — but aud, iss and exp do not depend on
// the transport, and an id_token minted for a different application would otherwise be
// accepted as this one's.
func TestGoogleRejectsIDTokensThatAreNotForThisClient(t *testing.T) {
	now := time.Now()

	cases := map[string]func(map[string]any){
		"another application's audience": func(claims map[string]any) {
			claims["aud"] = "999-someone-else.apps.googleusercontent.com"
		},
		"an issuer that is not google": func(claims map[string]any) {
			claims["iss"] = "https://accounts.evil.test"
		},
		"expired": func(claims map[string]any) {
			claims["exp"] = now.Add(-time.Minute).Unix()
		},
		"no expiry at all": func(claims map[string]any) {
			delete(claims, "exp")
		},
		"no subject": func(claims map[string]any) {
			delete(claims, "sub")
		},
	}

	for name, corrupt := range cases {
		claims := validGoogleClaims(now)
		corrupt(claims)
		stub := newGoogleStub(t, unsignedIDToken(t, claims))
		provider := newTestGoogleProvider(t, stub.server.URL, now)

		if _, err := provider.Exchange(context.Background(), "code", "verifier"); err == nil {
			t.Errorf("%s: Exchange() succeeded", name)
		}
	}
}

// Google uses the bare form as well as the https one, and rejecting it would break
// sign-in for whichever accounts get served it.
func TestGoogleAcceptsBothIssuerForms(t *testing.T) {
	now := time.Now()
	for _, issuer := range []string{"https://accounts.google.com", "accounts.google.com"} {
		claims := validGoogleClaims(now)
		claims["iss"] = issuer
		stub := newGoogleStub(t, unsignedIDToken(t, claims))
		provider := newTestGoogleProvider(t, stub.server.URL, now)

		if _, err := provider.Exchange(context.Background(), "code", "verifier"); err != nil {
			t.Errorf("issuer %q was rejected: %v", issuer, err)
		}
	}
}

func TestGoogleRejectsAMalformedIDToken(t *testing.T) {
	for name, idToken := range map[string]string{
		"not a jwt":        "just-a-string",
		"two parts":        "header.payload",
		"unparseable body": "aGVhZGVy.bm90LWpzb24.c2ln",
	} {
		stub := newGoogleStub(t, idToken)
		provider := newTestGoogleProvider(t, stub.server.URL, time.Now())
		if _, err := provider.Exchange(context.Background(), "code", "verifier"); err == nil {
			t.Errorf("%s: Exchange() succeeded", name)
		}
	}
}

// An unverified address is not an error at this layer: the provider is reporting a fact,
// and the decision about what to do with it belongs to OAuthService, which refuses it.
// Turning it into an error here would move that decision somewhere it cannot be seen.
func TestGoogleReportsAnUnverifiedAddressWithoutFailing(t *testing.T) {
	now := time.Now()
	claims := validGoogleClaims(now)
	claims["email_verified"] = false
	stub := newGoogleStub(t, unsignedIDToken(t, claims))
	provider := newTestGoogleProvider(t, stub.server.URL, now)

	identity, err := provider.Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if identity.EmailVerified {
		t.Error("email_verified was false in the token but true in the identity")
	}
}

func TestEmailVerifiedDecodesFromBothJSONForms(t *testing.T) {
	cases := map[string]bool{
		`true`:    true,
		`"true"`:  true,
		`false`:   false,
		`"false"`: false,
		`null`:    false,
	}
	for raw, want := range cases {
		var value flexibleBool
		if err := value.UnmarshalJSON([]byte(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if bool(value) != want {
			t.Errorf("%s decoded to %v, want %v", raw, bool(value), want)
		}
	}

	var value flexibleBool
	if err := value.UnmarshalJSON([]byte(`"maybe"`)); err == nil {
		t.Error("a non-boolean decoded without an error")
	}
}

func TestNewGoogleProviderRejectsMissingConfiguration(t *testing.T) {
	valid := GoogleConfig{
		ClientID:     testGoogleClientID,
		ClientSecret: testGoogleClientSecret,
		RedirectURL:  testGoogleRedirectURL,
	}
	for name, remove := range map[string]func(*GoogleConfig){
		"no client id":     func(c *GoogleConfig) { c.ClientID = "" },
		"no client secret": func(c *GoogleConfig) { c.ClientSecret = "" },
		"no redirect url":  func(c *GoogleConfig) { c.RedirectURL = " " },
	} {
		configuration := valid
		remove(&configuration)
		if _, err := NewGoogleProvider(configuration); err == nil {
			t.Errorf("%s: NewGoogleProvider() succeeded", name)
		}
	}
}

// The default client must have a timeout. http.DefaultClient does not, and a token
// endpoint that accepts the connection and never answers would hold the request until
// the browser gives up.
func TestTheDefaultGoogleClientHasATimeout(t *testing.T) {
	provider, err := NewGoogleProvider(GoogleConfig{
		ClientID: testGoogleClientID, ClientSecret: testGoogleClientSecret,
		RedirectURL: testGoogleRedirectURL,
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider() error = %v", err)
	}
	if provider.client.Timeout <= 0 {
		t.Error("the token exchange client has no timeout")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
