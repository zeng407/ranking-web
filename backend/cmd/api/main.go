package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/authoring"
	"2pick.app/backend/internal/comments"
	"2pick.app/backend/internal/config"
	"2pick.app/backend/internal/gameplay"
	"2pick.app/backend/internal/gameroom"
	"2pick.app/backend/internal/httpapi"
	"2pick.app/backend/internal/ingest"
	"2pick.app/backend/internal/media"
	"2pick.app/backend/internal/platform/mysqlstore"
	"2pick.app/backend/internal/platform/redisstore"
	"2pick.app/backend/internal/postaccess"
	"2pick.app/backend/internal/publiccontent"
	"2pick.app/backend/internal/queue"
	"2pick.app/backend/internal/ranking"

	"github.com/redis/go-redis/v9"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	configuration, err := config.Load()
	if err != nil {
		logger.Error("configuration_error", "error", err)
		os.Exit(1)
	}

	var ready atomic.Bool
	var authVerifier auth.TokenVerifier
	var database *sql.DB
	var publicContent publiccontent.Repository
	var gameRepository gameplay.Repository
	var commentRepository comments.Repository
	if len(configuration.Auth.PublicKey) > 0 {
		verifier, err := auth.NewVerifier(auth.Config{
			Issuer:           configuration.Auth.Issuer,
			Audience:         configuration.Auth.Audience,
			PublicKey:        configuration.Auth.PublicKey,
			MaxTokenLifetime: configuration.Auth.MaxTokenLifetime,
			ClockSkew:        configuration.Auth.ClockSkew,
		})
		if err != nil {
			logger.Error("auth_configuration_error", "error", err)
			os.Exit(1)
		}
		authVerifier = verifier
		logger.Info("auth_bridge_enabled", "issuer", configuration.Auth.Issuer, "audience", configuration.Auth.Audience)
	}
	if configuration.Database.Enabled() {
		database, err = mysqlstore.Open(configuration.Database)
		if err != nil {
			logger.Error("database_configuration_error", "error", err)
			os.Exit(1)
		}
		defer database.Close()
		publicContent = publiccontent.NewMySQLRepository(database)
		gameRepository = gameplay.NewMySQLRepository(database)
		commentRepository = comments.NewMySQLRepository(database)
		logger.Info("public_content_database_enabled", "host", configuration.Database.Host, "database", configuration.Database.Name)
	}

	// Redis, only for the rank freshness flag a finished game sets. The API served
	// every request without it before this, so it stays optional: no Redis means the
	// flag is not written and httpapi logs that, rather than the process refusing to
	// start over a feature that touches one code path.
	//
	// LARAVEL_CACHE_PREFIX is not optional in the same way. The key is shared with
	// Laravel's own cache, so the wrong prefix writes a flag nothing reads — which is
	// indistinguishable from not writing one.
	// Opened once and shared: the rank freshness flag and the OAuth state store both
	// want it, and two clients would mean two connection pools to the same server.
	var redisClient *redis.Client
	if configuration.Redis.Enabled() {
		redisClient = redisstore.Open(configuration.Redis)
		defer redisClient.Close()
	}

	var rankFreshness ranking.FreshnessStore
	if redisClient != nil {
		laravelCachePrefix := os.Getenv("LARAVEL_CACHE_PREFIX")
		store, err := ranking.NewRedisFreshness(redisClient, laravelCachePrefix)
		if err != nil {
			logger.Error("rank_freshness_configuration_error", "error", err)
			os.Exit(1)
		}
		rankFreshness = store
		if laravelCachePrefix == "" {
			logger.Warn("laravel_cache_prefix_unset",
				"effect", "the freshness flag is written under a prefix Laravel does not read",
				"fix", "set LARAVEL_CACHE_PREFIX to Laravel's full cache key prefix")
		}
		logger.Info("rank_freshness_enabled", "redis", configuration.Redis.Addr, "prefix", laravelCachePrefix)
	}

	// The API's own login, which needs the private half of the signing key. Without
	// it the process still verifies tokens Laravel issued, so this stays optional
	// exactly like the verifier is: the auth endpoints answer 503 and nothing else
	// changes.
	var authService *auth.Service
	if len(configuration.Auth.PrivateKey) > 0 && database != nil {
		issuer, err := auth.NewIssuer(auth.IssuerConfig{
			Issuer:     configuration.Auth.Issuer,
			Audience:   configuration.Auth.Audience,
			PrivateKey: configuration.Auth.PrivateKey,
			TTL:        auth.DefaultAccessTokenTTL,
		})
		if err != nil {
			logger.Error("auth_issuer_configuration_error", "error", err)
			os.Exit(1)
		}
		service, err := auth.NewService(auth.ServiceOptions{
			Users:      auth.NewMySQLUserStore(database),
			Registrar:  auth.NewMySQLUserStore(database),
			Accounts:   auth.NewMySQLAccountStore(database),
			Avatars:    avatarStore(configuration, logger),
			Sessions:   auth.NewMySQLRefreshStore(database),
			Issuer:     issuer,
			Logger:     logger,
			RefreshTTL: configuration.Auth.RefreshTTL,
			Timezone:   applicationTimezone(logger),
		})
		if err != nil {
			logger.Error("auth_service_configuration_error", "error", err)
			os.Exit(1)
		}
		authService = service
		logger.Info("auth_login_enabled",
			"issuer", configuration.Auth.Issuer,
			"refresh_ttl", configuration.Auth.RefreshTTL.String())
	}

	// The account behind a token, so /api/v1/auth/me can replace the `user` half of
	// Laravel's /session-context. Needs only the database, not the signing key: a
	// process that merely verifies Laravel's tokens can still answer it.
	var profiles auth.ProfileStore
	if database != nil {
		profiles = auth.NewMySQLProfileStore(database)
	}

	// Google sign-in. Needs the session service, because a completed flow issues the
	// same grant a password login does — so it is off whenever login is off, whatever
	// the OAuth variables say.
	var oauthService httpapi.OAuthService
	if authService != nil && configuration.GoogleOAuth.Configured() {
		provider, err := auth.NewGoogleProvider(auth.GoogleConfig{
			ClientID:     configuration.GoogleOAuth.ClientID,
			ClientSecret: configuration.GoogleOAuth.ClientSecret,
			RedirectURL:  configuration.GoogleOAuth.RedirectURL,
		})
		if err != nil {
			logger.Error("oauth_provider_configuration_error", "provider", "google", "error", err)
			os.Exit(1)
		}

		// Redis when there is one. The in-process store cannot serve more than one
		// instance: the callback can land on a different replica than the one that
		// started the flow, and every such sign-in would fail with an invalid state.
		var states auth.OAuthStateStore
		if redisClient != nil {
			states = auth.NewRedisOAuthStates(redisClient)
		} else {
			states = auth.NewMemoryOAuthStates()
			logger.Warn("oauth_state_store_in_memory",
				"effect", "sign-in breaks as soon as this API runs more than one instance",
				"fix", "set REDIS_ADDR for the api process")
		}

		service, err := auth.NewOAuthService(auth.OAuthServiceOptions{
			Provider: provider,
			States:   states,
			Social:   auth.NewMySQLSocialStore(database),
			Sessions: authService,
			Logger:   logger,
			// The origins the browser may call this API from are the ones it may be
			// returned to. One list, so the two cannot disagree — and a disagreement
			// would be an open redirect on the URL that follows a sign-in.
			ReturnAllowlist: httpapi.OAuthReturnAllowlist(configuration.AllowedOrigins),
			DefaultReturnTo: httpapi.OAuthDefaultReturnTo(configuration.AllowedOrigins),
		})
		if err != nil {
			logger.Error("oauth_service_configuration_error", "provider", "google", "error", err)
			os.Exit(1)
		}
		oauthService = service
		logger.Info("oauth_enabled", "provider", "google",
			"redirect_url", configuration.GoogleOAuth.RedirectURL)
	}

	// Game rooms. The API owns the request side only — joining, wagering, renaming; the
	// worker owns settlement and the broadcast. Both halves read the same tables, which is
	// why the unique indexes from migrations 00010 to 00012 matter: without them a
	// double-submitted wager becomes two rows and gets counted twice.
	var (
		gameRooms         httpapi.GameRoomService
		gameRoomReader    httpapi.GameRoomReader
		gameRoomBoard     httpapi.GameRoomLeaderboard
		gameRoomAnnouncer httpapi.GameRoomAnnouncer
	)
	if database != nil {
		participationRepository := gameroom.NewMySQLParticipation(database)

		// Redis when available: the rename cooldown has to be shared, or N API instances
		// allow N renames per window.
		var limiter gameroom.RenameLimiter
		if redisClient != nil {
			limiter = gameroom.NewRedisRenameLimiter(redisClient)
		} else {
			limiter = gameroom.NewMemoryRenameLimiter()
			logger.Warn("game_room_rename_limiter_in_memory",
				"effect", "the rename cooldown is per instance, so N instances allow N renames per window",
				"fix", "set REDIS_ADDR for the api process")
		}

		participation, err := gameroom.NewParticipation(gameroom.ParticipationOptions{
			Repository: participationRepository,
			Limiter:    limiter,
			Scoring:    gameroom.DefaultScoring(),
		})
		if err != nil {
			logger.Error("game_room_configuration_error", "error", err)
			os.Exit(1)
		}

		gameRooms = participation
		gameRoomReader = participationRepository
		gameRoomBoard = gameroom.NewMySQLRepository(database, gameroom.DefaultScoring())

		// The API publishes for the first time here. A host's votes decide matches, and
		// each decided match settles the wagers a room placed on it — the worker has
		// consumed those messages since D6, but nothing produced them, so every room's
		// leaderboard stood still. Needs Redis: without it the vote path records rounds
		// and the room never learns about them, which New() warns about.
		if redisClient != nil {
			transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
			if err != nil {
				logger.Error("queue_transport_configuration_error", "error", err)
				os.Exit(1)
			}
			publisher, err := queue.NewPublisher(transport)
			if err != nil {
				logger.Error("queue_publisher_configuration_error", "error", err)
				os.Exit(1)
			}
			announcer, err := gameroom.NewAnnouncer(participationRepository, publisher, logger)
			if err != nil {
				logger.Error("game_room_announcer_configuration_error", "error", err)
				os.Exit(1)
			}
			gameRoomAnnouncer = announcer
		}

		logger.Info("game_rooms_enabled", "announces_settlements", gameRoomAnnouncer != nil)
	}

	readiness := func() bool {
		if !ready.Load() {
			return false
		}
		if database == nil {
			return true
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return database.PingContext(ctx) == nil
	}

	// The post editor. Needs the database; the rank refresher it queues work through is
	// optional, and without it a deletion leaves a stale report for the daily schedule
	// to correct rather than refusing to delete.
	var authoringService httpapi.AuthoringService
	if database != nil {
		repository := authoring.NewMySQLRepository(database)
		options := authoring.ServiceOptions{
			Posts: repository, Elements: repository, Passwords: repository,
		}
		if redisClient != nil {
			transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
			if err != nil {
				logger.Error("queue_transport_configuration_error", "error", err)
				os.Exit(1)
			}
			publisher, err := queue.NewPublisher(transport)
			if err != nil {
				logger.Error("queue_publisher_configuration_error", "error", err)
				os.Exit(1)
			}
			refresher, err := authoring.NewQueueRankRefresher(publisher)
			if err != nil {
				logger.Error("authoring_rank_refresher_configuration_error", "error", err)
				os.Exit(1)
			}
			options.Ranks = refresher
		}
		service, err := authoring.NewService(options)
		if err != nil {
			logger.Error("authoring_service_configuration_error", "error", err)
			os.Exit(1)
		}
		authoringService = service
		logger.Info("post_editor_enabled", "queues_rank_refresh", options.Ranks != nil)
	}

	// Adding media to a post. Needs the object store as well as the database, so it is
	// wired separately from the editor: an api without the AWS_* variables still edits
	// posts and answers 503 on the upload endpoint alone.
	var ingestService httpapi.IngestService
	if database != nil && configuration.Storage.Enabled() {
		objects, err := media.NewS3Store(media.NewS3Client(media.ClientConfig{
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
			logger.Error("ingest_object_store_configuration_error", "error", err)
			os.Exit(1)
		}
		fetcher := media.NewFetcher()
		scraper, err := ingest.NewPageScraper(fetcher)
		if err != nil {
			logger.Error("ingest_page_scraper_configuration_error", "error", err)
			os.Exit(1)
		}
		options := ingest.ServiceOptions{
			Store:   ingest.NewMySQLStore(database),
			Objects: objects,
			// The URL side. The fetcher and the prober both refuse private addresses,
			// which is what makes it safe to point them at a URL an author pasted.
			Fetcher: fetcher,
			Prober:  ingest.NewHeadProber(),
			Pages:   scraper,
		}
		if configuration.YouTubeAPIKey != "" {
			youtube, err := ingest.NewYouTubeAPI(configuration.YouTubeAPIKey)
			if err != nil {
				logger.Error("ingest_youtube_configuration_error", "error", err)
				os.Exit(1)
			}
			options.YouTube = youtube
		} else {
			logger.Warn("youtube_imports_disabled", "reason", "YOUTUBE_API_KEY is not set")
		}
		if redisClient != nil {
			transport, err := queue.NewRedisTransport(redisClient, queue.DefaultKeyPrefix)
			if err != nil {
				logger.Error("queue_transport_configuration_error", "error", err)
				os.Exit(1)
			}
			publisher, err := queue.NewPublisher(transport)
			if err != nil {
				logger.Error("queue_publisher_configuration_error", "error", err)
				os.Exit(1)
			}
			thumbnailer, err := ingest.NewQueueThumbnailer(publisher)
			if err != nil {
				logger.Error("ingest_thumbnailer_configuration_error", "error", err)
				os.Exit(1)
			}
			options.Thumbs = thumbnailer

			limiter, err := ingest.NewRedisRateLimiter(redisClient, "")
			if err != nil {
				logger.Error("ingest_rate_limiter_configuration_error", "error", err)
				os.Exit(1)
			}
			options.Limiter = limiter
		}
		service, err := ingest.NewService(options)
		if err != nil {
			logger.Error("ingest_service_configuration_error", "error", err)
			os.Exit(1)
		}
		ingestService = service
		logger.Info("media_uploads_enabled",
			"queues_thumbnails", options.Thumbs != nil, "rate_limited", options.Limiter != nil,
			"youtube_imports", options.YouTube != nil)
	} else if database != nil {
		logger.Warn("media_uploads_disabled", "reason", "AWS_BUCKET and AWS_URL are not configured")
	}

	// The door code on a password-protected post. Needs the database for the digest and
	// the signing key for the proof; without the key it stays off, and protected posts
	// remain invisible to this API rather than being opened by an unsigned token.
	var postAccessService httpapi.PostAccessService
	if database != nil && len(configuration.Auth.PrivateKey) > 0 {
		signer, err := postaccess.NewSigner(configuration.Auth.PrivateKey.Seed())
		if err != nil {
			logger.Error("post_access_signer_configuration_error", "error", err)
			os.Exit(1)
		}
		options := postaccess.ServiceOptions{
			Store: postaccess.NewMySQLStore(database), Signer: signer,
		}
		if redisClient != nil {
			attempts, err := postaccess.NewRedisAttempts(redisClient, "")
			if err != nil {
				logger.Error("post_access_rate_limiter_configuration_error", "error", err)
				os.Exit(1)
			}
			options.Attempts = attempts
		} else {
			// Worth a warning: a door code is short and shared, so an unlimited endpoint
			// is the one place here where guessing is cheap.
			logger.Warn("post_access_not_rate_limited", "reason", "REDIS_ADDR is not configured")
		}
		service, err := postaccess.NewService(options)
		if err != nil {
			logger.Error("post_access_service_configuration_error", "error", err)
			os.Exit(1)
		}
		postAccessService = service
		logger.Info("post_access_enabled", "rate_limited", options.Attempts != nil)
	} else if database != nil {
		logger.Warn("post_access_disabled",
			"reason", "GO_AUTH_PRIVATE_KEY is not set, so there is no key to sign access tokens with",
			"effect", "password-protected posts are not readable through this API")
	}

	handler := httpapi.New(httpapi.Options{
		ServiceName:       "ranking-api",
		Version:           version,
		Commit:            commit,
		Environment:       configuration.Environment,
		AllowedOrigins:    configuration.AllowedOrigins,
		Ready:             readiness,
		Logger:            logger,
		AuthVerifier:      authVerifier,
		PublicContent:     publicContent,
		Gameplay:          gameRepository,
		Comments:          commentRepository,
		Authoring:         authoringService,
		Ingest:            ingestService,
		PostAccess:        postAccessService,
		RankFreshness:     rankFreshness,
		AuthService:       authService,
		OAuthService:      oauthService,
		Profiles:          profiles,
		GameRooms:         gameRooms,
		GameRoomReader:    gameRoomReader,
		GameRoomBoard:     gameRoomBoard,
		GameRoomAnnouncer: gameRoomAnnouncer,
	})

	server := &http.Server{
		Addr:              configuration.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server_starting", "address", configuration.HTTPAddr, "environment", configuration.Environment, "version", version)
		ready.Store(true)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server_failed", "error", err)
			os.Exit(1)
		}
	case <-rootContext.Done():
		ready.Store(false)
		logger.Info("server_stopping")
		shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("server_shutdown_failed", "error", err)
			_ = server.Close()
			os.Exit(1)
		}
	}

	logger.Info("server_stopped")
}

// applicationTimezone is config/app.php's timezone, which the name-change limit is
// written against: its rule is "not already changed today", and which day that is
// depends on where midnight falls.
//
// Hard-coded rather than an environment variable because Laravel hard-codes it too. A
// tzdata-less image would fail to load it, so a failure falls back to UTC with a warning
// rather than refusing to start — the effect is a limit that resets eight hours early.
func applicationTimezone(logger *slog.Logger) *time.Location {
	location, err := time.LoadLocation(applicationTimezoneName)
	if err != nil {
		logger.Warn("application_timezone_unavailable",
			"timezone", applicationTimezoneName, "error", err, "effect", "day boundaries fall at UTC midnight")
		return time.UTC
	}
	return location
}

const applicationTimezoneName = "Asia/Taipei"

// avatarStore is where uploaded avatars go. Nil when the bucket is not configured, which
// leaves the upload endpoint answering 503 while the rest of the settings work.
func avatarStore(configuration config.Config, logger *slog.Logger) auth.AvatarStore {
	if !configuration.Storage.Enabled() {
		logger.Warn("avatar_uploads_disabled", "reason", "AWS_BUCKET and AWS_URL are not configured")
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
		logger.Warn("avatar_uploads_disabled", "error", err)
		return nil
	}
	return store
}
