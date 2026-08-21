// Command worker consumes background jobs.
//
// It consumes and also publishes: the rank history reorder pass fans out one
// assignment message per pending date.
//
// Registered handlers cover the ranking pipeline, media processing and the
// multiplayer game rooms. Media and game rooms register only when their
// prerequisites are present — ffmpeg and an object store, and a Pusher-protocol
// server — so a message of an unregistered type is dead-lettered rather than
// silently dropped.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/gameroom"
	"2pick.app/backend/internal/jobs"
	"2pick.app/backend/internal/media"
	"2pick.app/backend/internal/platform/health"
	"2pick.app/backend/internal/platform/mysqlstore"
	"2pick.app/backend/internal/platform/redislock"
	"2pick.app/backend/internal/platform/redisstore"
	"2pick.app/backend/internal/posttrend"
	"2pick.app/backend/internal/publicpost"
	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/ranking"
	"2pick.app/backend/internal/realtime"
	"2pick.app/backend/internal/sitemap"
	"github.com/redis/go-redis/v9"
)

var (
	version = "dev"
	commit  = "unknown"
)

const serviceName = "ranking-worker"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("worker_failed", "error", err)
		os.Exit(1)
	}
	logger.Info("worker_stopped")
}

func run(logger *slog.Logger) error {
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// A worker with no queue backend or no database cannot do any of its work, so
	// it fails fast instead of idling while looking healthy.
	if !configuration.Redis.Enabled() {
		return errors.New("REDIS_ADDR is required by the worker")
	}
	if !configuration.Database.Enabled() {
		return errors.New("DB_HOST is required by the worker")
	}

	database, err := mysqlstore.Open(configuration.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer database.Close()

	redisClient := redisstore.Open(configuration.Redis)
	defer redisClient.Close()

	transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
	if err != nil {
		return fmt.Errorf("queue transport: %w", err)
	}
	locker, err := redislock.New(redisClient, jobs.LockKeyPrefix)
	if err != nil {
		return fmt.Errorf("worker lock: %w", err)
	}
	// The worker publishes as well as consumes: the reorder pass fans out one rank
	// assignment per pending date.
	publisher, err := queue.NewPublisher(transport)
	if err != nil {
		return fmt.Errorf("queue publisher: %w", err)
	}

	// Laravel prefixes every cache key with
	// "{slug(APP_NAME)}_database_{slug(APP_NAME)}_cache:", so reading or writing an
	// entry it also uses requires that prefix. Two things need it: the element rank
	// memo, and the freshness flag that only Laravel currently sets.
	laravelCachePrefix := os.Getenv("LARAVEL_CACHE_PREFIX")
	statsStore, err := ranking.NewRedisStats(redisClient, laravelCachePrefix)
	if err != nil {
		return fmt.Errorf("rank stats store: %w", err)
	}
	freshness, err := ranking.NewRedisFreshness(redisClient, laravelCachePrefix)
	if err != nil {
		return fmt.Errorf("rank freshness store: %w", err)
	}
	if laravelCachePrefix == "" {
		// Without it the daily history sweep sees no flagged posts, because the flag
		// is written by Laravel's UpdatePostRank listener under its own prefix.
		logger.Warn("laravel_cache_prefix_unset",
			"effect", "the rank history sweep will not see posts flagged by Laravel",
			"fix", "set LARAVEL_CACHE_PREFIX to Laravel's full cache key prefix")
	}
	pendingDates, err := ranking.NewRedisPendingDates(redisClient, ranking.PendingDatesKeyPrefix)
	if err != nil {
		return fmt.Errorf("rank history pending dates: %w", err)
	}
	rankingService, err := ranking.NewService(ranking.Options{
		Repository:   ranking.NewMySQLRepository(database),
		Reports:      ranking.NewMySQLReportRepository(database),
		History:      ranking.NewMySQLHistoryRepository(database),
		HistoryRanks: ranking.NewMySQLHistoryRankRepository(database),
		Posts:        ranking.NewMySQLPostRepository(database),
		Freshness:    freshness,
		Pending:      pendingDates,
		Stats:        statsStore,
		Logger:       logger,
		// record_date is a DATE and Laravel's today() uses the application
		// timezone, so this must not fall back to the container's UTC.
		Location: configuration.Scheduler.Timezone,
	})
	if err != nil {
		return fmt.Errorf("ranking service: %w", err)
	}

	// The hot-post trends need no backend beyond the database, so they register
	// unconditionally. Four schedules dispatch post_trend.create, and until these
	// existed every one of those messages would have dead-lettered.
	trendService, err := posttrend.NewService(posttrend.Options{
		Repository: posttrend.NewMySQLRepository(database),
		Publisher:  publisher,
		Logger:     logger,
		// The windows are DATE values and Laravel's today() uses the application
		// timezone, so this must not fall back to the container's UTC: for eight hours
		// of every day it would land on the previous date.
		Location: configuration.Scheduler.Timezone,
	})
	if err != nil {
		return fmt.Errorf("post trend service: %w", err)
	}

	// The public post listing. It shares the freshness flag and the per-post resource
	// cache with Laravel, so it needs the cache prefix; without it the debounce would
	// be Go-only and the PHP endpoints would keep serving the payloads this replaces.
	publicPostService, err := newPublicPostService(database, redisClient, laravelCachePrefix,
		configuration, logger)
	if err != nil {
		return fmt.Errorf("public post service: %w", err)
	}

	registry := jobs.NewRegistry()
	registry.MustRegister(
		rankingService.Registration(),
		rankingService.ReportRegistration(),
		rankingService.HistoryRegistration(),
		// The reorder pass fans out one assignment per pending date, so it needs
		// the publisher.
		rankingService.ReorderRegistration(publisher),
		rankingService.AssignRegistration(),
		rankingService.PurgeRegistration(),
		// The daily sweeps enumerate posts and fan out, so they need the publisher.
		rankingService.SweepPostHistoryRegistration(publisher),
		rankingService.BuildPostHistoryRegistration(publisher),
		rankingService.SweepPurgeHistoryRegistration(publisher),
		// The create pass publishes its own follow-up, so the two are separate
		// registrations rather than one handler doing both: a failure while assigning
		// positions must not force the play counts to be recomputed.
		trendService.CreateRegistration(),
		trendService.PositionsRegistration(),
		publicPostService.Registration(),
	)

	// Media handlers need ffmpeg and an object store. Both are present only in the
	// media-target image, so a worker built without them registers no media
	// handlers and dead-letters those messages rather than failing at startup.
	if err := registerMediaHandlers(registry, publisher, database, configuration, logger); err != nil {
		return err
	}

	// Game room handlers need Soketi, since their whole output is a broadcast.
	if err := registerGameRoomHandlers(registry, publisher, database, redisClient,
		laravelCachePrefix, configuration, logger); err != nil {
		return err
	}

	jobRunner, err := jobs.NewRunner(jobs.RunnerOptions{
		Reserver:    transport,
		Registry:    registry,
		Locker:      locker,
		Logger:      logger,
		Queues:      configuration.Worker.Queues,
		Concurrency: configuration.Worker.Concurrency,
		JobTimeout:  configuration.Worker.JobTimeout,
	})
	if err != nil {
		return fmt.Errorf("job runner: %w", err)
	}

	var ready atomic.Bool
	healthServer := health.NewServer(health.Options{
		Addr:        configuration.Worker.HealthAddr,
		ServiceName: serviceName,
		Version:     version,
		Commit:      commit,
		Environment: configuration.Environment,
		Ready:       readiness(&ready, database, redisClient),
		Logger:      logger,
	})

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	healthErrors := make(chan error, 1)
	go func() {
		logger.Info("worker_health_starting", "address", configuration.Worker.HealthAddr)
		healthErrors <- healthServer.ListenAndServe()
	}()

	logger.Info("worker_starting",
		"environment", configuration.Environment,
		"version", version,
		"queues", configuration.Worker.Queues,
		"concurrency", configuration.Worker.Concurrency,
		"job_timeout", configuration.Worker.JobTimeout.String(),
		"timezone", configuration.Scheduler.Timezone.String(),
		"registered_handlers", registry.Len(),
		"handler_types", registry.Types(),
	)

	consumeDone := make(chan struct{})
	go func() {
		defer close(consumeDone)
		if err := jobRunner.Run(rootContext); err != nil {
			logger.Error("worker_consume_failed", "error", err)
		}
	}()
	ready.Store(true)

	select {
	case err := <-healthErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server: %w", err)
		}
	case <-rootContext.Done():
		logger.Info("worker_stopping")
	}

	ready.Store(false)
	shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
	defer cancel()

	// Wait for in-flight handlers before tearing anything down. Run() returns once
	// every handler has finished, and each one already has its own timeout, so a
	// job that is mid-write is not cut off.
	select {
	case <-consumeDone:
	case <-shutdownContext.Done():
		logger.Warn("worker_drain_incomplete", "timeout", configuration.ShutdownTimeout.String())
	}

	processed, failed, retried, deadLettered, deferred := jobRunner.Stats()
	logger.Info("worker_totals",
		"processed", processed, "failed", failed, "retried", retried,
		"dead_lettered", deadLettered, "deferred", deferred,
	)

	if err := healthServer.Shutdown(shutdownContext); err != nil {
		_ = healthServer.Close()
		return fmt.Errorf("health shutdown: %w", err)
	}
	return nil
}

// readiness reports the worker ready only once it is running and both backends
// answer, so a rolling deploy does not route work to a task that cannot process
// it.
func readiness(ready *atomic.Bool, database *sql.DB, redisClient *redis.Client) health.ReadyFunc {
	return func(ctx context.Context) error {
		if !ready.Load() {
			return errors.New("worker is not started")
		}
		probeContext, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := database.PingContext(probeContext); err != nil {
			return fmt.Errorf("database unreachable: %w", err)
		}
		if err := redisstore.Ping(probeContext, redisClient); err != nil {
			return fmt.Errorf("redis unreachable: %w", err)
		}
		return nil
	}
}

// registerMediaHandlers wires the media jobs when the process can actually run
// them.
//
// The requirements are ffmpeg on PATH and a configured object store. The api and
// scheduler images ship FROM scratch with no ffmpeg, and a deployment may run a
// worker fleet without storage credentials, so this reports what is missing and
// carries on rather than refusing to start. Messages of those types then reach the
// dead-letter queue, which is visible, instead of being silently dropped.
func registerMediaHandlers(
	registry *jobs.Registry,
	publisher *queue.Publisher,
	database *sql.DB,
	configuration config.Config,
	logger *slog.Logger,
) error {
	if !configuration.Storage.Enabled() {
		logger.Warn("media_handlers_not_registered", "reason", "AWS_BUCKET and AWS_URL are not configured")
		return nil
	}

	transcoder, err := media.NewTranscoder()
	if err != nil {
		logger.Warn("media_handlers_not_registered", "reason", "ffmpeg unavailable", "error", err)
		return nil
	}

	store, err := media.NewS3Store(media.NewS3Client(media.ClientConfig{
		Endpoint:     configuration.Storage.Endpoint,
		Region:       configuration.Storage.Region,
		AccessKey:    configuration.Storage.AccessKey,
		SecretKey:    configuration.Storage.SecretKey,
		UsePathStyle: configuration.Storage.UsePathStyle,
	}), media.S3Config{
		Bucket:    configuration.Storage.Bucket,
		PublicURL: configuration.Storage.PublicURL,
	})
	if err != nil {
		return fmt.Errorf("media object store: %w", err)
	}

	mediaService, err := media.NewThumbnailService(media.ServiceOptions{
		Elements:   media.NewMySQLElementRepository(database),
		Store:      store,
		Transcoder: transcoder,
		Logger:     logger,
	})
	if err != nil {
		return fmt.Errorf("media service: %w", err)
	}

	registry.MustRegister(
		mediaService.ImageRegistration(),
		mediaService.VideoRegistration(),
		// The sweep fans out one job per element, so it needs the publisher.
		mediaService.SweepRegistration(publisher),
		mediaService.RemoveDeletedFilesRegistration(),
	)

	// The sitemap writes through the same bucket.
	if err := registerSitemapHandler(registry, database, store, configuration, logger); err != nil {
		return err
	}

	if media.PrivateSourcesAllowed() {
		// Loud on purpose. With this on, a user-submitted URL can point the worker at
		// anything inside the VPC, including the metadata endpoint.
		logger.Warn("media_private_source_fetching_enabled",
			"env", media.AllowPrivateSourcesEnv,
			"risk", "user-supplied URLs may reach internal addresses; never set this in production")
	}

	logger.Info("media_handlers_registered",
		"bucket", configuration.Storage.Bucket,
		"public_url", configuration.Storage.PublicURL,
		"path_style", configuration.Storage.UsePathStyle,
	)
	return nil
}

// registerSitemapHandler wires sitemap generation when the process can store the
// result.
//
// It shares the media object store, so it needs the same AWS_* configuration. The
// key is separate because the sitemap belongs at the root of whatever serves the
// frontend, not under the thumbnail prefixes.
func registerSitemapHandler(
	registry *jobs.Registry,
	database *sql.DB,
	store sitemap.Writer,
	configuration config.Config,
	logger *slog.Logger,
) error {
	baseURL := os.Getenv("SITEMAP_BASE_URL")
	if baseURL == "" {
		logger.Warn("sitemap_handler_not_registered", "reason", "SITEMAP_BASE_URL is not configured")
		return nil
	}

	generator, err := sitemap.NewGenerator(sitemap.Options{
		Repository:   sitemap.NewMySQLRepository(database),
		Writer:       store,
		Logger:       logger,
		BaseURL:      baseURL,
		ObjectKey:    envOrDefault("SITEMAP_OBJECT_KEY", "sitemap.xml"),
		HomeImageURL: os.Getenv("SITEMAP_HOME_IMAGE_URL"),
	})
	if err != nil {
		return fmt.Errorf("sitemap generator: %w", err)
	}

	registry.MustRegister(generator.Registration())
	logger.Info("sitemap_handler_registered",
		"base_url", baseURL,
		"object_key", envOrDefault("SITEMAP_OBJECT_KEY", "sitemap.xml"),
		"bucket", configuration.Storage.Bucket,
	)
	return nil
}

// registerGameRoomHandlers wires the multiplayer leaderboard jobs.
//
// They need a Pusher-protocol server: the recompute is worthless if the result
// cannot reach the room. A worker with no PUSHER_* configuration therefore registers
// nothing and lets those messages dead-letter, which is visible, rather than
// recomputing standings that no browser ever sees.
func registerGameRoomHandlers(
	registry *jobs.Registry,
	publisher *queue.Publisher,
	database *sql.DB,
	redisClient *redis.Client,
	laravelCachePrefix string,
	configuration config.Config,
	logger *slog.Logger,
) error {
	if !configuration.Realtime.Enabled() {
		logger.Warn("game_room_handlers_not_registered",
			"reason", "PUSHER_APP_ID, PUSHER_APP_KEY and PUSHER_APP_SECRET are not configured")
		return nil
	}

	broadcaster, err := realtime.NewPusherPublisher(realtime.Config{
		AppID:   configuration.Realtime.AppID,
		Key:     configuration.Realtime.Key,
		Secret:  configuration.Realtime.Secret,
		Host:    configuration.Realtime.Host,
		Port:    configuration.Realtime.Port,
		Secure:  configuration.Realtime.Secure,
		Cluster: configuration.Realtime.Cluster,
	})
	if err != nil {
		return fmt.Errorf("game room broadcaster: %w", err)
	}

	tracker, err := gameroom.NewRedisTracker(redisClient)
	if err != nil {
		return fmt.Errorf("game room refresh tracker: %w", err)
	}
	legacyCache, err := gameroom.NewRedisLegacyCache(redisClient, laravelCachePrefix)
	if err != nil {
		return fmt.Errorf("game room legacy cache: %w", err)
	}
	if laravelCachePrefix == "" {
		// Without it the deletes miss, and GameController keeps serving a cached
		// leaderboard and reporting rank_updating for the flag's whole lifetime.
		logger.Warn("game_room_legacy_cache_prefix_unset",
			"effect", "the PHP room endpoints will serve stale standings",
			"fix", "set LARAVEL_CACHE_PREFIX while GameController still serves them")
	}

	service, err := gameroom.NewService(gameroom.Options{
		Repository:  gameroom.NewMySQLRepository(database, gameroom.DefaultScoring()),
		Tracker:     tracker,
		Legacy:      legacyCache,
		Broadcaster: broadcaster,
		// The same reader the API serves the room's REST endpoints from, so the pairing a
		// participant is pushed and the pairing they would have polled are one query.
		Votes:     gameroom.NewMySQLParticipation(database),
		Publisher: publisher,
		Scoring:   gameroom.DefaultScoring(),
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("game room service: %w", err)
	}

	registry.MustRegister(service.SettleRegistration(), service.RefreshRegistration(),
		service.RoundRegistration())
	logger.Info("game_room_handlers_registered",
		"pusher_host", configuration.Realtime.Host,
		"pusher_port", configuration.Realtime.Port,
		"secure", configuration.Realtime.Secure,
	)
	return nil
}

// newPublicPostService wires the listing refresh.
//
// PUBLIC_POST_CONTINUOUS selects between the two pacing models. Off, the refresh shares
// Laravel's public_post_fresh flag, so it rebuilds when a post changes and otherwise at
// most every ten minutes — which is what the PHP does today. On, there is no debounce
// and the only thing pacing the work is the overlap lock, so a run starts again as soon
// as the previous one finishes.
func newPublicPostService(
	database *sql.DB,
	redisClient *redis.Client,
	laravelCachePrefix string,
	configuration config.Config,
	logger *slog.Logger,
) (*publicpost.Service, error) {
	continuous, err := boolFromEnv("PUBLIC_POST_CONTINUOUS")
	if err != nil {
		return nil, err
	}

	var freshness publicpost.FreshnessStore
	if continuous {
		freshness = publicpost.AlwaysStale{}
	} else {
		shared, err := publicpost.NewRedisFreshness(redisClient, laravelCachePrefix)
		if err != nil {
			return nil, err
		}
		freshness = shared
	}

	cache, err := publicpost.NewRedisResourceCache(redisClient, laravelCachePrefix)
	if err != nil {
		return nil, err
	}

	logger.Info("public_post_handler_registered",
		"continuous", continuous,
		"chunk_size", publicpost.ChunkSize,
		// Stated in the log because it is the change most likely to surprise: the PHP
		// stopped at 2,000 posts per pass.
		"batch_cap", "none",
	)

	return publicpost.NewService(publicpost.Options{
		Repository: publicpost.NewMySQLRepository(database),
		Freshness:  freshness,
		Cache:      cache,
		Logger:     logger,
		// The trend windows are DATE values and Laravel's today() uses the application
		// timezone.
		Location: configuration.Scheduler.Timezone,
	})
}

// boolFromEnv reads an optional flag, defaulting to false and rejecting anything it
// cannot read rather than silently treating it as off.
func boolFromEnv(key string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "", "false", "0", "off":
		return false, nil
	case "true", "1", "on":
		return true, nil
	}
	return false, fmt.Errorf("%s: %q is not a boolean", key, value)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
