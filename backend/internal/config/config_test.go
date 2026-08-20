package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("GO_AUTH_PUBLIC_KEY", "")
	t.Setenv("GO_AUTH_ISSUER", "")
	t.Setenv("GO_AUTH_AUDIENCE", "")
	t.Setenv("GO_AUTH_MAX_TOKEN_TTL", "")
	t.Setenv("GO_AUTH_CLOCK_SKEW", "")
	t.Setenv("DB_HOST", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Environment != "local" {
		t.Fatalf("Environment = %q", configuration.Environment)
	}
	if configuration.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", configuration.HTTPAddr)
	}
	if configuration.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s", configuration.ShutdownTimeout)
	}
	if len(configuration.Auth.PublicKey) != 0 || configuration.Auth.Audience != "2pick-go-api" {
		t.Fatalf("Auth = %#v", configuration.Auth)
	}
	if configuration.Database.Enabled() {
		t.Fatalf("Database = %#v", configuration.Database)
	}
	if configuration.AdultSignInRequired {
		t.Fatalf("AdultSignInRequired = true, want adult posts playable without an account by default")
	}
}

// The adult gate is the one setting that changes who may play a post, so both the
// switch and a typo in it are worth pinning: a value nobody can read must not quietly
// leave the gate open.
func TestLoadAdultSignInRequired(t *testing.T) {
	t.Setenv("ADULT_CONTENT_REQUIRE_SIGN_IN", "true")
	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !configuration.AdultSignInRequired {
		t.Fatalf("AdultSignInRequired = false, want true")
	}

	t.Setenv("ADULT_CONTENT_REQUIRE_SIGN_IN", "yes")
	if _, err := Load(); err == nil {
		t.Fatalf("Load() error = nil, want a rejection of an unreadable boolean")
	}
}

func TestLoadDatabaseConfig(t *testing.T) {
	t.Setenv("DB_HOST", "mysql")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_DATABASE", "ranking")
	t.Setenv("DB_USERNAME", "reader")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_MAX_OPEN_CONNS", "12")
	t.Setenv("DB_MAX_IDLE_CONNS", "6")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Database.Host != "mysql" || configuration.Database.Port != 3307 {
		t.Fatalf("Database = %#v", configuration.Database)
	}
	if configuration.Database.MaxOpenConns != 12 || configuration.Database.MaxIdleConns != 6 {
		t.Fatalf("Database pool = %#v", configuration.Database)
	}
}

func TestLoadRejectsPartialDatabaseConfig(t *testing.T) {
	t.Setenv("DB_HOST", "mysql")
	t.Setenv("DB_DATABASE", "")
	t.Setenv("DB_USERNAME", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject partial database configuration")
	}
}

func TestLoadRejectsInvalidAuthPublicKey(t *testing.T) {
	t.Setenv("GO_AUTH_PUBLIC_KEY", "not-a-valid-ed25519-key")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject an invalid auth public key")
	}
}

func TestLoadRejectsWildcardCredentialOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "*")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject wildcard origins")
	}
}

func TestLoadParsesOriginsAndTimeout(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://2pick.app, https://app.2pick.app/,https://2pick.app")
	t.Setenv("SHUTDOWN_TIMEOUT", "15s")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(configuration.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins = %#v", configuration.AllowedOrigins)
	}
	if configuration.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s", configuration.ShutdownTimeout)
	}
}

func TestLoadRedisDisabledWithoutAddr(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Redis.Enabled() {
		t.Fatalf("Redis = %#v", configuration.Redis)
	}
}

func TestLoadRedisConfig(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_POOL_SIZE", "25")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !configuration.Redis.Enabled() {
		t.Fatal("Redis should be enabled when REDIS_ADDR is set")
	}
	if configuration.Redis.Addr != "redis:6379" || configuration.Redis.DB != 3 {
		t.Fatalf("Redis = %#v", configuration.Redis)
	}
	if configuration.Redis.Password != "secret" || configuration.Redis.PoolSize != 25 {
		t.Fatalf("Redis = %#v", configuration.Redis)
	}
	if configuration.Redis.DialTimeout != 5*time.Second {
		t.Fatalf("Redis.DialTimeout = %s", configuration.Redis.DialTimeout)
	}
}

// Laravel's REDIS_HOST is a bare hostname and Go's client wants host:port, so a
// value copied straight across must fail loudly instead of dialling nothing.
func TestLoadRejectsRedisAddrWithoutPort(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject REDIS_ADDR without a port")
	}
}

func TestLoadRejectsRedisAddrAsURL(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis://redis:6379")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject a URL in REDIS_ADDR")
	}
}

func TestLoadWorkerDefaults(t *testing.T) {
	t.Setenv("WORKER_QUEUES", "")
	t.Setenv("WORKER_CONCURRENCY", "")
	t.Setenv("WORKER_JOB_TIMEOUT", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Worker.Concurrency != 4 || configuration.Worker.HealthAddr != ":8081" {
		t.Fatalf("Worker = %#v", configuration.Worker)
	}
	// Must stay within Laravel's retry_after: 90 so a job is never handed to a
	// second consumer while the first is still holding it.
	if configuration.Worker.JobTimeout != 90*time.Second {
		t.Fatalf("Worker.JobTimeout = %s", configuration.Worker.JobTimeout)
	}
	if len(configuration.Worker.Queues) != 3 || configuration.Worker.Queues[0] != "high" {
		t.Fatalf("Worker.Queues = %#v", configuration.Worker.Queues)
	}
}

// Queue order is consumption priority, so it must survive parsing verbatim.
func TestLoadWorkerQueuesPreserveOrder(t *testing.T) {
	t.Setenv("WORKER_QUEUES", "rank_report_history, game_room ,default")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	expected := []string{"rank_report_history", "game_room", "default"}
	if len(configuration.Worker.Queues) != len(expected) {
		t.Fatalf("Worker.Queues = %#v", configuration.Worker.Queues)
	}
	for index, queue := range expected {
		if configuration.Worker.Queues[index] != queue {
			t.Fatalf("Worker.Queues[%d] = %q, want %q", index, configuration.Worker.Queues[index], queue)
		}
	}
}

// A duplicate makes the intended priority ambiguous, so it is an error rather
// than something to silently collapse.
func TestLoadRejectsDuplicateWorkerQueues(t *testing.T) {
	t.Setenv("WORKER_QUEUES", "default,high,default")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject duplicate queues")
	}
}

func TestLoadRejectsEmptyWorkerQueueList(t *testing.T) {
	t.Setenv("WORKER_QUEUES", " , ,")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject a queue list that resolves to nothing")
	}
}

// The release images are FROM scratch with no system zoneinfo, so this also
// proves the embedded tzdata is linked in.
func TestLoadSchedulerTimezoneDefaultsToTaipei(t *testing.T) {
	t.Setenv("SCHEDULER_TIMEZONE", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Scheduler.Timezone == nil {
		t.Fatal("Scheduler.Timezone must never be nil")
	}
	if configuration.Scheduler.Timezone.String() != "Asia/Taipei" {
		t.Fatalf("Scheduler.Timezone = %q", configuration.Scheduler.Timezone)
	}
	if configuration.Scheduler.HealthAddr != ":8082" {
		t.Fatalf("Scheduler.HealthAddr = %q", configuration.Scheduler.HealthAddr)
	}
}

func TestLoadRejectsUnknownSchedulerTimezone(t *testing.T) {
	t.Setenv("SCHEDULER_TIMEZONE", "Mars/Olympus_Mons")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject an unknown timezone")
	}
}

// "Local" resolves without error but silently inherits the container's zone,
// which is exactly the ambiguity cron expressions cannot tolerate.
func TestLoadRejectsLocalSchedulerTimezone(t *testing.T) {
	t.Setenv("SCHEDULER_TIMEZONE", "Local")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject the Local timezone")
	}
}

func TestLoadRealtimeConfig(t *testing.T) {
	t.Setenv("PUSHER_APP_ID", "two-pick-pusher")
	t.Setenv("PUSHER_APP_KEY", "two-pick-app-key")
	t.Setenv("PUSHER_APP_SECRET", "two-pick-pw98321")
	t.Setenv("PUSHER_HOST", "soketi")
	t.Setenv("PUSHER_PORT", "6001")
	t.Setenv("PUSHER_SCHEME", "http")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !configuration.Realtime.Enabled() {
		t.Fatal("Realtime should be enabled when the PUSHER_* variables are set")
	}
	if configuration.Realtime.Host != "soketi" || configuration.Realtime.Port != "6001" {
		t.Errorf("Realtime = %+v, want host soketi port 6001", configuration.Realtime)
	}
	if configuration.Realtime.Secure {
		t.Error("PUSHER_SCHEME=http must not enable TLS")
	}
}

func TestLoadRealtimeDefaultsMatchBroadcastingConfig(t *testing.T) {
	t.Setenv("PUSHER_APP_ID", "id")
	t.Setenv("PUSHER_APP_KEY", "key")
	t.Setenv("PUSHER_APP_SECRET", "secret")
	t.Setenv("PUSHER_HOST", "")
	t.Setenv("PUSHER_PORT", "")
	t.Setenv("PUSHER_SCHEME", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Same fallbacks as config/broadcasting.php.
	if configuration.Realtime.Host != "127.0.0.1" || configuration.Realtime.Port != "6001" {
		t.Errorf("Realtime = %+v, want the broadcasting.php defaults", configuration.Realtime)
	}
}

func TestLoadRealtimeTreatsHTTPSAsSecure(t *testing.T) {
	t.Setenv("PUSHER_APP_ID", "id")
	t.Setenv("PUSHER_APP_KEY", "key")
	t.Setenv("PUSHER_APP_SECRET", "secret")
	t.Setenv("PUSHER_SCHEME", "HTTPS")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !configuration.Realtime.Secure {
		t.Error("PUSHER_SCHEME=HTTPS must enable TLS regardless of case")
	}
}

// Partial configuration is the dangerous case: a publisher built with a key and no
// secret authenticates against nothing, and the symptom would be rooms that stop
// updating rather than an error.
func TestLoadRejectsPartialRealtimeConfig(t *testing.T) {
	for name, set := range map[string]map[string]string{
		"no secret": {"PUSHER_APP_ID": "id", "PUSHER_APP_KEY": "key", "PUSHER_APP_SECRET": ""},
		"no key":    {"PUSHER_APP_ID": "id", "PUSHER_APP_KEY": "", "PUSHER_APP_SECRET": "secret"},
		"no app id": {"PUSHER_APP_ID": "", "PUSHER_APP_KEY": "key", "PUSHER_APP_SECRET": "secret"},
	} {
		t.Run(name, func(t *testing.T) {
			for key, value := range set {
				t.Setenv(key, value)
			}
			if _, err := Load(); err == nil {
				t.Errorf("Load() accepted the %q case", name)
			}
		})
	}
}

func TestRealtimeDisabledWhenUnset(t *testing.T) {
	t.Setenv("PUSHER_APP_ID", "")
	t.Setenv("PUSHER_APP_KEY", "")
	t.Setenv("PUSHER_APP_SECRET", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Realtime.Enabled() {
		t.Error("Realtime should be disabled when nothing is configured")
	}
}

// mailEnvironment clears every variable loadMailConfig reads, so a value left in the
// developer's own environment cannot make one of these pass or fail by accident.
func mailEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GO_MAIL_TRANSPORT", "MAIL_HOST", "MAIL_PORT", "MAIL_ENCRYPTION",
		"MAIL_USERNAME", "MAIL_PASSWORD", "MAIL_FROM_ADDRESS", "MAIL_FROM_NAME",
		"APP_URL", "APP_NAME",
	} {
		t.Setenv(key, "")
	}
}

func TestMailDisabledWithoutTransport(t *testing.T) {
	mailEnvironment(t)
	// The variables Laravel needs are set, and they must not be read as intent: the
	// worker and the scheduler run with the same environment and send nothing.
	t.Setenv("MAIL_HOST", "smtp.gmail.com")
	t.Setenv("MAIL_PORT", "587")
	t.Setenv("MAIL_FROM_ADDRESS", "2pick.app@gmail.com")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Mail.Configured() {
		t.Error("mail should be off until GO_MAIL_TRANSPORT is set")
	}
}

func TestLoadMailSMTPConfig(t *testing.T) {
	mailEnvironment(t)
	t.Setenv("GO_MAIL_TRANSPORT", "smtp")
	t.Setenv("MAIL_HOST", "smtp.gmail.com")
	t.Setenv("MAIL_PORT", "587")
	t.Setenv("MAIL_ENCRYPTION", "TLS")
	t.Setenv("MAIL_USERNAME", "2pick.app@gmail.com")
	t.Setenv("MAIL_PASSWORD", "an-app-password")
	t.Setenv("MAIL_FROM_ADDRESS", "2pick.app@gmail.com")
	t.Setenv("MAIL_FROM_NAME", "殘酷二選一")
	t.Setenv("APP_URL", "https://2pick.app/")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	mail := configuration.Mail
	if !mail.Configured() {
		t.Fatal("mail should be configured")
	}
	if mail.Host != "smtp.gmail.com" || mail.Port != 587 {
		t.Errorf("relay = %s:%d", mail.Host, mail.Port)
	}
	if mail.Encryption != "tls" {
		t.Errorf("Encryption = %q, want the value lower-cased", mail.Encryption)
	}
	if mail.FromName != "殘酷二選一" {
		t.Errorf("FromName = %q", mail.FromName)
	}
	// The trailing slash is dropped so links do not end up with a double slash.
	if mail.AppURL != "https://2pick.app" {
		t.Errorf("AppURL = %q", mail.AppURL)
	}
}

func TestLoadMailResolvesTheLaravelFromNamePlaceholder(t *testing.T) {
	mailEnvironment(t)
	t.Setenv("GO_MAIL_TRANSPORT", "log")
	t.Setenv("APP_URL", "http://localhost")
	t.Setenv("APP_NAME", "殘酷二選一")
	// What .env holds verbatim. Compose passes it through without expanding it.
	t.Setenv("MAIL_FROM_NAME", "${APP_NAME}")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Mail.FromName != "殘酷二選一" {
		t.Errorf("FromName = %q, want the expanded application name", configuration.Mail.FromName)
	}
}

func TestLoadRejectsMailConfigurationThatCannotSend(t *testing.T) {
	cases := map[string]map[string]string{
		"an unknown transport": {"GO_MAIL_TRANSPORT": "sendmail", "APP_URL": "http://localhost"},
		"no APP_URL to link to": {
			"GO_MAIL_TRANSPORT": "smtp", "MAIL_HOST": "smtp.gmail.com",
			"MAIL_PORT": "587", "MAIL_FROM_ADDRESS": "2pick.app@gmail.com",
		},
		"smtp without a host": {
			"GO_MAIL_TRANSPORT": "smtp", "APP_URL": "http://localhost",
			"MAIL_PORT": "587", "MAIL_FROM_ADDRESS": "2pick.app@gmail.com",
		},
		"smtp without a port": {
			"GO_MAIL_TRANSPORT": "smtp", "APP_URL": "http://localhost",
			"MAIL_HOST": "smtp.gmail.com", "MAIL_FROM_ADDRESS": "2pick.app@gmail.com",
		},
		"smtp with nobody to send as": {
			"GO_MAIL_TRANSPORT": "smtp", "APP_URL": "http://localhost",
			"MAIL_HOST": "smtp.gmail.com", "MAIL_PORT": "587",
		},
	}

	for name, environment := range cases {
		t.Run(name, func(t *testing.T) {
			mailEnvironment(t)
			for key, value := range environment {
				t.Setenv(key, value)
			}
			if _, err := Load(); err == nil {
				t.Errorf("Load() accepted the %q case", name)
			}
		})
	}
}
