package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	// The scheduler resolves an explicit IANA timezone, and the release images are
	// built FROM scratch with no system zoneinfo. Embedding the database keeps
	// timezone handling identical across local, container and production runs.
	_ "time/tzdata"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second
	defaultAuthIssuer      = "http://localhost"
	defaultAuthAudience    = "2pick-go-api"
	defaultMaxTokenTTL     = 10 * time.Minute
	defaultAuthClockSkew   = 30 * time.Second
	defaultDatabasePort    = 3306
	defaultDBMaxOpenConns  = 20
	defaultDBMaxIdleConns  = 20
	defaultDBConnLifetime  = 3 * time.Minute
	defaultDBConnIdleTime  = time.Minute

	defaultRedisAddr         = ""
	defaultRedisDB           = 0
	defaultRedisDialTimeout  = 5 * time.Second
	defaultRedisReadTimeout  = 3 * time.Second
	defaultRedisWriteTimeout = 3 * time.Second
	defaultRedisPoolSize     = 10

	defaultWorkerHealthAddr  = ":8081"
	defaultWorkerConcurrency = 4
	// Laravel's redis queue connection uses retry_after: 90. The Go worker must
	// finish or extend a reservation inside the same window so a job is not
	// handed to a second consumer while it is still running.
	defaultWorkerJobTimeout = 90 * time.Second

	defaultSchedulerHealthAddr = ":8082"
	// Mirrors config/app.php 'timezone' => 'Asia/Taipei'. Cron expressions are
	// meaningless without this, and the container default is UTC.
	defaultSchedulerTimezone = "Asia/Taipei"
)

// defaultWorkerQueues is the consumption order, highest priority first. Names
// match the queues the Laravel jobs already declare.
var defaultWorkerQueues = []string{"high", "default", "low"}

type AuthConfig struct {
	// PrivateKey lets this process ISSUE access tokens, not just verify them. Empty
	// while Laravel is still the only issuer; set, the Go API can log a user in on
	// its own. Same GO_AUTH_PRIVATE_KEY value the PHP side reads, so tokens from
	// either issuer verify against the one public key.
	PrivateKey ed25519.PrivateKey
	// RefreshTTL is how long a session survives without being used.
	RefreshTTL       time.Duration
	Issuer           string
	Audience         string
	PublicKey        ed25519.PublicKey
	MaxTokenLifetime time.Duration
	ClockSkew        time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (config DatabaseConfig) Enabled() bool {
	return config.Host != ""
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

func (config RedisConfig) Enabled() bool {
	return config.Addr != ""
}

type WorkerConfig struct {
	HealthAddr  string
	Concurrency int
	Queues      []string
	JobTimeout  time.Duration
}

type SchedulerConfig struct {
	HealthAddr string
	Timezone   *time.Location
}

// StorageConfig points the media jobs at the object store.
//
// The names mirror Laravel's AWS_* variables so one .env can drive both during the
// cutover. PublicURL is AWS_URL, which is what Storage::url() prefixes.
type StorageConfig struct {
	Bucket    string
	Region    string
	PublicURL string
	// Endpoint overrides the AWS endpoint, for MinIO locally.
	Endpoint string
	// UsePathStyle is required by MinIO; real S3 uses virtual-host addressing.
	UsePathStyle bool
	AccessKey    string
	SecretKey    string
}

// Enabled reports whether the media jobs can run. A worker without storage would
// generate thumbnails it cannot store.
func (config StorageConfig) Enabled() bool {
	return config.Bucket != "" && config.PublicURL != ""
}

// RealtimeConfig points at the Pusher-protocol server, which is Soketi.
//
// The names mirror Laravel's PUSHER_* variables so one .env drives both during the
// cutover, and so the browser keeps talking to the same server through pusher-js.
type RealtimeConfig struct {
	AppID  string
	Key    string
	Secret string
	Host   string
	Port   string
	// Secure is PUSHER_SCHEME=https. Soketi runs plain HTTP behind the load
	// balancer locally, so this is off by default.
	Secure  bool
	Cluster string
}

// Enabled reports whether broadcasts can be sent. A worker without it cannot run
// the game room jobs, whose whole output is a broadcast.
func (config RealtimeConfig) Enabled() bool {
	return config.AppID != "" && config.Key != "" && config.Secret != "" && config.Host != ""
}

type Config struct {
	Environment     string
	HTTPAddr        string
	AllowedOrigins  []string
	ShutdownTimeout time.Duration
	Auth            AuthConfig
	Database        DatabaseConfig
	Redis           RedisConfig
	Worker          WorkerConfig
	Scheduler       SchedulerConfig
	Storage         StorageConfig
	// YouTubeAPIKey reads video metadata for URL imports. Empty leaves YouTube URLs
	// reporting themselves unavailable while every other source still works, which is
	// what a deployment without the key should do.
	YouTubeAPIKey string
	Realtime      RealtimeConfig
	GoogleOAuth   GoogleOAuthConfig
	// AdminAssetDir is the directory the back office's own build sits in.
	//
	// IT MUST NOT BE INSIDE THE PUBLIC DOCUMENT ROOT. The files are served by this
	// process, to a request that proves the admin role, and by nothing else — see
	// httpapi.serveAdminAsset. Empty leaves /admin/ answering 404, which is what a
	// deployment that does not host the back office should do.
	AdminAssetDir string
}

// GoogleOAuthConfig is the Google sign-in client.
//
// Reads the same GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET Laravel does, so both
// stacks can run the flow during the transition — Google allows several redirect URIs
// per client, and the Go callback is a different path than the PHP one.
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL must match a URI registered in the Google Cloud console exactly,
	// including scheme and port: Google compares it as a string, not as a URL.
	RedirectURL string
}

// Configured reports whether the sign-in can run.
func (config GoogleOAuthConfig) Configured() bool {
	return config.ClientID != "" && config.ClientSecret != "" && config.RedirectURL != ""
}

func Load() (Config, error) {
	shutdownTimeout, err := durationFromEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	allowedOrigins, err := parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"))
	if err != nil {
		return Config{}, err
	}
	authPublicKey, err := parseAuthPublicKey(os.Getenv("GO_AUTH_PUBLIC_KEY"))
	if err != nil {
		return Config{}, err
	}
	authPrivateKey, err := parseAuthPrivateKey(os.Getenv("GO_AUTH_PRIVATE_KEY"))
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := durationFromEnv("GO_AUTH_REFRESH_TTL", defaultRefreshTTL)
	if err != nil {
		return Config{}, err
	}
	maxTokenLifetime, err := durationFromEnv("GO_AUTH_MAX_TOKEN_TTL", defaultMaxTokenTTL)
	if err != nil {
		return Config{}, err
	}
	clockSkew, err := nonNegativeDurationFromEnv("GO_AUTH_CLOCK_SKEW", defaultAuthClockSkew)
	if err != nil {
		return Config{}, err
	}
	database, err := loadDatabaseConfig()
	if err != nil {
		return Config{}, err
	}
	redis, err := loadRedisConfig()
	if err != nil {
		return Config{}, err
	}
	worker, err := loadWorkerConfig()
	if err != nil {
		return Config{}, err
	}
	scheduler, err := loadSchedulerConfig()
	if err != nil {
		return Config{}, err
	}
	storage, err := loadStorageConfig()
	if err != nil {
		return Config{}, err
	}
	realtime, err := loadRealtimeConfig()
	if err != nil {
		return Config{}, err
	}
	googleOAuth, err := loadGoogleOAuthConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Environment:     stringFromEnv("APP_ENV", "local"),
		HTTPAddr:        stringFromEnv("HTTP_ADDR", defaultHTTPAddr),
		AllowedOrigins:  allowedOrigins,
		ShutdownTimeout: shutdownTimeout,
		Auth: AuthConfig{
			Issuer:           stringFromEnv("GO_AUTH_ISSUER", defaultAuthIssuer),
			Audience:         stringFromEnv("GO_AUTH_AUDIENCE", defaultAuthAudience),
			PublicKey:        authPublicKey,
			PrivateKey:       authPrivateKey,
			RefreshTTL:       refreshTTL,
			MaxTokenLifetime: maxTokenLifetime,
			ClockSkew:        clockSkew,
		},
		Database:  database,
		Redis:     redis,
		Worker:    worker,
		Scheduler: scheduler,
		Storage:   storage,
		// The same variable name Laravel reads it from.
		YouTubeAPIKey: strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")),
		Realtime:      realtime,
		GoogleOAuth:   googleOAuth,
		AdminAssetDir: strings.TrimSpace(os.Getenv("ADMIN_ASSET_DIR")),
	}, nil
}

// loadGoogleOAuthConfig reads the Google client.
//
// THE SWITCH IS GO_OAUTH_GOOGLE_REDIRECT_URL, NOT THE PRESENCE OF A CLIENT ID.
//
// The realtime and storage blocks treat "any variable set" as the opt-in, because
// PUSHER_* and AWS_* mean nothing to a process that is not using them. That rule
// cannot be applied here: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are already in
// every environment file because Laravel's Socialite needs them, so treating them as
// intent would make this block fire everywhere and then reject the configuration for
// missing a redirect URL — breaking the startup of every Go process in the stack,
// including the worker and the scheduler, which have nothing to do with sign-in.
//
// The redirect URL is the only variable that is specific to this implementation, so it
// is the one that says "run OAuth here". Set it and the id and secret become required;
// leave it and the endpoints answer 503.
//
// It has no default on purpose. A default would have to guess the public origin, and a
// redirect URI that does not match the Google console byte for byte fails with
// redirect_uri_mismatch — which reads as a bug in this code rather than as a missing
// variable.
func loadGoogleOAuthConfig() (GoogleOAuthConfig, error) {
	redirectURL := strings.TrimSpace(os.Getenv("GO_OAUTH_GOOGLE_REDIRECT_URL"))
	if redirectURL == "" {
		return GoogleOAuthConfig{}, nil
	}

	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	for name, value := range map[string]string{
		"GOOGLE_CLIENT_ID":     clientID,
		"GOOGLE_CLIENT_SECRET": clientSecret,
	} {
		if value == "" {
			return GoogleOAuthConfig{}, fmt.Errorf(
				"google oauth configuration: %s is required when GO_OAUTH_GOOGLE_REDIRECT_URL is set", name)
		}
	}

	return GoogleOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}, nil
}

// loadRealtimeConfig reads the PUSHER_* variables Laravel already defines.
//
// Partial configuration is rejected rather than ignored. An app id with no secret
// would produce a publisher that authenticates against nothing, and the failure
// would surface as rooms that quietly stop updating rather than as a startup error.
func loadRealtimeConfig() (RealtimeConfig, error) {
	appID := strings.TrimSpace(os.Getenv("PUSHER_APP_ID"))
	key := strings.TrimSpace(os.Getenv("PUSHER_APP_KEY"))
	secret := strings.TrimSpace(os.Getenv("PUSHER_APP_SECRET"))
	if appID == "" && key == "" && secret == "" {
		return RealtimeConfig{}, nil
	}
	for name, value := range map[string]string{
		"PUSHER_APP_ID":     appID,
		"PUSHER_APP_KEY":    key,
		"PUSHER_APP_SECRET": secret,
	} {
		if value == "" {
			return RealtimeConfig{}, fmt.Errorf("realtime configuration: %s is required when the others are set", name)
		}
	}

	return RealtimeConfig{
		AppID:  appID,
		Key:    key,
		Secret: secret,
		// Same defaults as config/broadcasting.php.
		Host:    stringFromEnv("PUSHER_HOST", "127.0.0.1"),
		Port:    stringFromEnv("PUSHER_PORT", "6001"),
		Secure:  strings.EqualFold(strings.TrimSpace(os.Getenv("PUSHER_SCHEME")), "https"),
		Cluster: strings.TrimSpace(os.Getenv("PUSHER_APP_CLUSTER")),
	}, nil
}

// loadStorageConfig reads the AWS_* variables Laravel already defines.
func loadStorageConfig() (StorageConfig, error) {
	bucket := strings.TrimSpace(os.Getenv("AWS_BUCKET"))
	if bucket == "" {
		return StorageConfig{}, nil
	}

	publicURL := strings.TrimSpace(os.Getenv("AWS_URL"))
	if publicURL == "" {
		// Without it a stored object has no URL to record, so the thumbnail would
		// be uploaded and then lost.
		return StorageConfig{}, fmt.Errorf("storage configuration: AWS_URL is required when AWS_BUCKET is set")
	}

	pathStyle, err := boolFromEnv("AWS_USE_PATH_STYLE_ENDPOINT", false)
	if err != nil {
		return StorageConfig{}, err
	}

	return StorageConfig{
		Bucket:       bucket,
		Region:       stringFromEnv("AWS_DEFAULT_REGION", "ap-east-2"),
		PublicURL:    publicURL,
		Endpoint:     strings.TrimSpace(os.Getenv("AWS_ENDPOINT")),
		UsePathStyle: pathStyle,
		AccessKey:    os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}, nil
}

func boolFromEnv(key string, fallback bool) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "":
		return fallback, nil
	case "true", "1", "on":
		return true, nil
	case "false", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean, got %q", key, value)
	}
}

func loadRedisConfig() (RedisConfig, error) {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		return RedisConfig{}, nil
	}
	if strings.Contains(addr, "://") {
		return RedisConfig{}, fmt.Errorf("redis configuration: REDIS_ADDR must be host:port, not a URL: %q", addr)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || port == "" {
		return RedisConfig{}, fmt.Errorf("redis configuration: REDIS_ADDR must be host:port: %q", addr)
	}

	db, err := nonNegativeIntFromEnv("REDIS_DB", defaultRedisDB)
	if err != nil {
		return RedisConfig{}, err
	}
	dialTimeout, err := durationFromEnv("REDIS_DIAL_TIMEOUT", defaultRedisDialTimeout)
	if err != nil {
		return RedisConfig{}, err
	}
	readTimeout, err := durationFromEnv("REDIS_READ_TIMEOUT", defaultRedisReadTimeout)
	if err != nil {
		return RedisConfig{}, err
	}
	writeTimeout, err := durationFromEnv("REDIS_WRITE_TIMEOUT", defaultRedisWriteTimeout)
	if err != nil {
		return RedisConfig{}, err
	}
	poolSize, err := positiveIntFromEnv("REDIS_POOL_SIZE", defaultRedisPoolSize)
	if err != nil {
		return RedisConfig{}, err
	}

	return RedisConfig{
		Addr:         addr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           db,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolSize:     poolSize,
	}, nil
}

func loadWorkerConfig() (WorkerConfig, error) {
	concurrency, err := positiveIntFromEnv("WORKER_CONCURRENCY", defaultWorkerConcurrency)
	if err != nil {
		return WorkerConfig{}, err
	}
	jobTimeout, err := durationFromEnv("WORKER_JOB_TIMEOUT", defaultWorkerJobTimeout)
	if err != nil {
		return WorkerConfig{}, err
	}
	queues, err := parseWorkerQueues(os.Getenv("WORKER_QUEUES"))
	if err != nil {
		return WorkerConfig{}, err
	}

	return WorkerConfig{
		HealthAddr:  stringFromEnv("WORKER_HEALTH_ADDR", defaultWorkerHealthAddr),
		Concurrency: concurrency,
		Queues:      queues,
		JobTimeout:  jobTimeout,
	}, nil
}

// parseWorkerQueues keeps the declared order because it is the consumption
// priority, so duplicates are rejected rather than silently collapsed.
func parseWorkerQueues(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		queues := make([]string, len(defaultWorkerQueues))
		copy(queues, defaultWorkerQueues)
		return queues, nil
	}

	queues := make([]string, 0)
	seen := make(map[string]struct{})
	for _, candidate := range strings.Split(value, ",") {
		queue := strings.TrimSpace(candidate)
		if queue == "" {
			continue
		}
		if _, exists := seen[queue]; exists {
			return nil, fmt.Errorf("WORKER_QUEUES contains a duplicate queue: %q", queue)
		}
		seen[queue] = struct{}{}
		queues = append(queues, queue)
	}
	if len(queues) == 0 {
		return nil, fmt.Errorf("WORKER_QUEUES must list at least one queue when set")
	}
	return queues, nil
}

func loadSchedulerConfig() (SchedulerConfig, error) {
	name := stringFromEnv("SCHEDULER_TIMEZONE", defaultSchedulerTimezone)
	location, err := time.LoadLocation(name)
	if err != nil {
		return SchedulerConfig{}, fmt.Errorf("SCHEDULER_TIMEZONE is not a known IANA timezone: %q", name)
	}
	if name == "Local" {
		return SchedulerConfig{}, fmt.Errorf("SCHEDULER_TIMEZONE must be an explicit IANA timezone, not %q", name)
	}

	return SchedulerConfig{
		HealthAddr: stringFromEnv("SCHEDULER_HEALTH_ADDR", defaultSchedulerHealthAddr),
		Timezone:   location,
	}, nil
}

func loadDatabaseConfig() (DatabaseConfig, error) {
	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	if host == "" {
		return DatabaseConfig{}, nil
	}

	name := strings.TrimSpace(os.Getenv("DB_DATABASE"))
	user := strings.TrimSpace(os.Getenv("DB_USERNAME"))
	if name == "" || user == "" {
		return DatabaseConfig{}, errorsForDatabase("DB_DATABASE and DB_USERNAME are required when DB_HOST is configured")
	}

	port, err := positiveIntFromEnv("DB_PORT", defaultDatabasePort)
	if err != nil {
		return DatabaseConfig{}, err
	}
	maxOpenConns, err := positiveIntFromEnv("DB_MAX_OPEN_CONNS", defaultDBMaxOpenConns)
	if err != nil {
		return DatabaseConfig{}, err
	}
	maxIdleConns, err := nonNegativeIntFromEnv("DB_MAX_IDLE_CONNS", defaultDBMaxIdleConns)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if maxIdleConns > maxOpenConns {
		return DatabaseConfig{}, errorsForDatabase("DB_MAX_IDLE_CONNS cannot exceed DB_MAX_OPEN_CONNS")
	}
	connMaxLifetime, err := durationFromEnv("DB_CONN_MAX_LIFETIME", defaultDBConnLifetime)
	if err != nil {
		return DatabaseConfig{}, err
	}
	connMaxIdleTime, err := nonNegativeDurationFromEnv("DB_CONN_MAX_IDLE_TIME", defaultDBConnIdleTime)
	if err != nil {
		return DatabaseConfig{}, err
	}

	return DatabaseConfig{
		Host:            host,
		Port:            port,
		Name:            name,
		User:            user,
		Password:        os.Getenv("DB_PASSWORD"),
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
	}, nil
}

func errorsForDatabase(message string) error {
	return fmt.Errorf("database configuration: %s", message)
}

func stringFromEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration: %q", key, value)
	}
	return duration, nil
}

func nonNegativeDurationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration: %q", key, value)
	}
	return duration, nil
}

func positiveIntFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer: %q", key, value)
	}
	return parsed, nil
}

func nonNegativeIntFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer: %q", key, value)
	}
	return parsed, nil
}

func parseAuthPublicKey(value string) (ed25519.PublicKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("GO_AUTH_PUBLIC_KEY must be base64 for a %d-byte Ed25519 public key", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(decoded), nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	origins := make([]string, 0)
	seen := make(map[string]struct{})
	for _, candidate := range strings.Split(value, ",") {
		origin := strings.TrimRight(strings.TrimSpace(candidate), "/")
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, fmt.Errorf("ALLOWED_ORIGINS cannot contain * when credentials are enabled")
		}

		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" {
			return nil, fmt.Errorf("ALLOWED_ORIGINS contains an invalid origin: %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	return origins, nil
}

// defaultRefreshTTL matches auth.RefreshTokenTTL. Duplicated as a constant rather
// than imported so the config package stays free of a dependency on auth.
const defaultRefreshTTL = 30 * 24 * time.Hour

// parseAuthPrivateKey reads GO_AUTH_PRIVATE_KEY.
//
// Accepts the two forms App\Services\Auth\GoAccessTokenService accepts: a 32-byte
// Ed25519 seed or a 64-byte private key, base64 encoded. Empty is valid and means
// this process only verifies tokens, which is the state before login moved off
// Laravel.
//
// A key of the wrong length is rejected rather than padded: it would produce tokens
// no verifier accepts, and the failure would only surface at the first login.
func parseAuthPrivateKey(encoded string) (ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("auth configuration: GO_AUTH_PRIVATE_KEY must be valid base64: %w", err)
		}
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, fmt.Errorf(
			"auth configuration: GO_AUTH_PRIVATE_KEY must be a %d-byte seed or a %d-byte key, got %d bytes",
			ed25519.SeedSize, ed25519.PrivateKeySize, len(raw))
	}
}
