package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"2pick.app/backend/internal/auth"
	"2pick.app/backend/internal/comments"
	"2pick.app/backend/internal/gameplay"
	"2pick.app/backend/internal/publiccontent"
	"2pick.app/backend/internal/ranking"
)

const requestIDHeader = "X-Request-ID"

type Options struct {
	ServiceName    string
	Version        string
	Commit         string
	Environment    string
	AllowedOrigins []string
	Ready          func() bool
	Now            func() time.Time
	Logger         *slog.Logger
	AuthVerifier   auth.TokenVerifier
	PublicContent  publiccontent.Repository
	Gameplay       gameplay.Repository
	Comments       comments.Repository
	// Authoring is the post editor. Optional: without it those endpoints answer 503.
	Authoring AuthoringService
	// Ingest adds media to a post. Optional, and separately so: an api with no object
	// store still edits posts, it just cannot take an upload.
	Ingest IngestService
	// Admin is the moderation back office. Optional: without it every /api/v1/admin
	// endpoint answers 503, which is the state before the back office moved off Laravel.
	Admin AdminService
	// AdminAssetDir holds the built admin bundle, which must NOT be on the public origin
	// — see admin_assets.go. Empty leaves /admin/ a 404 and the grant endpoints with it.
	AdminAssetDir string
	// AdminAssetKey signs the pass that gates that directory. Required for it to be
	// served at all; deriving it from the token signing key keeps it out of the
	// environment. Empty has the same effect as an empty AdminAssetDir.
	AdminAssetKey []byte
	// PostAccess checks a protected post's door code. Optional: without it the access
	// endpoint answers 503 and password posts stay invisible, which is how the API
	// behaved before this existed — that is, it fails closed.
	PostAccess PostAccessService
	// RankFreshness flags a post's ranks as stale when a game finishes, the job
	// App\Listeners\UpdatePostRank does in Laravel. Optional: without it the API
	// still serves every request, but the daily rank history sweep sees no posts,
	// so New() logs that rather than leaving it silent.
	RankFreshness ranking.FreshnessStore
	// AuthService issues and rotates the API's own sessions. Optional: without it the
	// /api/v1/auth/{login,refresh,logout} endpoints answer 503 and the rest of the API
	// is unaffected, which is the state before authentication moved off Laravel.
	AuthService AuthService
	// OAuthService runs the Google sign-in. Optional in the same way, and separately:
	// a deployment can have password login without an OAuth client configured.
	OAuthService OAuthService
	// OAuthFailureReturnTo is where a failed callback sends the browser. Defaults to
	// the first allowed origin, which is the SPA in every deployment so far.
	OAuthFailureReturnTo string
	// Profiles lets /api/v1/auth/me answer with the account behind the token, not just
	// the token's claims. Optional: without it the endpoint stays a pure token check.
	Profiles auth.ProfileStore
	// GameRooms and GameRoomReader serve the multiplayer betting rooms. Both are needed
	// together; with neither, those endpoints answer 503 and the rest of the API is
	// unaffected.
	GameRooms      GameRoomService
	GameRoomReader GameRoomReader
	// GameRoomBoard reads the standings, and is the settlement repository: the API only
	// reads that table, never recomputes it inside a request.
	GameRoomBoard GameRoomLeaderboard
	// GameRoomAnnouncer publishes the settlement work a host's votes create. Without it
	// a room's wagers are recorded and never resolved.
	GameRoomAnnouncer GameRoomAnnouncer
}

type api struct {
	serviceName       string
	version           string
	commit            string
	environment       string
	allowedOrigins    map[string]struct{}
	ready             func() bool
	now               func() time.Time
	logger            *slog.Logger
	authVerifier      auth.TokenVerifier
	rankFreshness     ranking.FreshnessStore
	authService       AuthService
	oauthService      OAuthService
	profiles          auth.ProfileStore
	gameRooms         GameRoomService
	gameRoomReader    GameRoomReader
	gameRoomBoard     GameRoomLeaderboard
	gameRoomAnnouncer GameRoomAnnouncer
	// oauthFailureReturnTo is where a failed OAuth callback sends the browser. A
	// failure has no validated per-flow target to fall back on — the state that held
	// it may be exactly what could not be read.
	oauthFailureReturnTo string
	publicContent        publiccontent.Repository
	gameplay             gameplay.Repository
	comments             comments.Repository
	// authoring is the post editor. Nil answers 503 on its endpoints, like every other
	// optional dependency here.
	authoring AuthoringService
	// ingest adds media to a post. Separate from authoring because it needs the object
	// store, which a process may be started without.
	ingest IngestService
	// admin is the moderation back office. Reachable only from behind requireAdmin.
	admin AdminService
	// The gated admin bundle. Both are needed together; either one empty means the
	// bundle is not served by this process.
	adminAssetDir string
	adminAssetKey []byte
	// postAccess checks a protected post's door code. Nil leaves every caller with the
	// public view.
	postAccess PostAccessService
}

type contextKey string

const requestIDContextKey contextKey = "request-id"

type envelope struct {
	Data  any     `json:"data,omitempty"`
	Meta  *meta   `json:"meta,omitempty"`
	Error *apiErr `json:"error,omitempty"`
}

type meta struct {
	RequestID string `json:"request_id"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(options Options) http.Handler {
	if options.ServiceName == "" {
		options.ServiceName = "ranking-api"
	}
	if options.Version == "" {
		options.Version = "dev"
	}
	if options.Commit == "" {
		options.Commit = "unknown"
	}
	if options.Environment == "" {
		options.Environment = "local"
	}
	if options.Ready == nil {
		options.Ready = func() bool { return true }
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Logger == nil {
		options.Logger = slog.New(slog.NewJSONHandler(httpWriter{}, nil))
	}

	allowedOrigins := make(map[string]struct{}, len(options.AllowedOrigins))
	for _, origin := range options.AllowedOrigins {
		allowedOrigins[strings.TrimRight(origin, "/")] = struct{}{}
	}

	server := &api{
		serviceName:       options.ServiceName,
		version:           options.Version,
		commit:            options.Commit,
		environment:       options.Environment,
		allowedOrigins:    allowedOrigins,
		ready:             options.Ready,
		now:               options.Now,
		logger:            options.Logger,
		authVerifier:      options.AuthVerifier,
		rankFreshness:     options.RankFreshness,
		authService:       options.AuthService,
		oauthService:      options.OAuthService,
		profiles:          options.Profiles,
		gameRooms:         options.GameRooms,
		gameRoomReader:    options.GameRoomReader,
		gameRoomBoard:     options.GameRoomBoard,
		gameRoomAnnouncer: options.GameRoomAnnouncer,
		// Falls back to the first allowed origin rather than to "/", which on this
		// API's own origin is a 404 rather than a page that can show the reason.
		oauthFailureReturnTo: firstNonEmpty(options.OAuthFailureReturnTo,
			firstAllowedOrigin(options.AllowedOrigins)),
		publicContent: options.PublicContent,
		gameplay:      options.Gameplay,
		comments:      options.Comments,
		authoring:     options.Authoring,
		ingest:        options.Ingest,
		postAccess:    options.PostAccess,
		admin:         options.Admin,
		adminAssetDir: strings.TrimSpace(options.AdminAssetDir),
		adminAssetKey: options.AdminAssetKey,
	}

	if server.adminAssetDir != "" && len(server.adminAssetKey) == 0 {
		// Loud, because the symptom is a back office that cannot be opened at all: the
		// directory is mounted, every pass fails to verify, and /admin/ answers 403 to a
		// moderator who is signed in correctly.
		options.Logger.Warn("admin_asset_key_unset",
			"effect", "the admin bundle is configured but no pass can be signed, so /admin/ answers 403 to everyone",
			"fix", "configure GO_AUTH_PRIVATE_KEY for the api process, which is what the pass key is derived from")
	}

	if options.GameRooms != nil && options.GameRoomAnnouncer == nil {
		// Loud, because the symptom is a room that draws correctly and never updates:
		// wagers are accepted, rounds are recorded, and the message that would settle
		// them is never published.
		options.Logger.Warn("game_room_announcer_unset",
			"effect", "wagers are recorded but never settled, so every room leaderboard stays still",
			"fix", "configure the queue publisher for the api process")
	}

	if options.RankFreshness == nil && options.Gameplay != nil {
		// Loud, because the symptom is invisible: games complete, the API answers
		// normally, and the daily rank history build simply finds nothing to do.
		options.Logger.Warn("rank_freshness_store_unset",
			"effect", "finished games will not flag their post, so the rank history sweep finds no posts",
			"fix", "configure REDIS_ADDR and LARAVEL_CACHE_PREFIX for the api process")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", method(http.MethodGet, server.live))
	mux.HandleFunc("/health/ready", method(http.MethodGet, server.readiness))
	mux.HandleFunc("/api/v1/system/info", method(http.MethodGet, server.systemInfo))
	mux.HandleFunc("/api/v1/auth/me", method(http.MethodGet, server.requireAuth(server.currentIdentity)))
	mux.HandleFunc("/api/v1/auth/login", method(http.MethodPost, server.login))
	mux.HandleFunc("/api/v1/auth/register", method(http.MethodPost, server.register))
	mux.HandleFunc("/api/v1/auth/refresh", method(http.MethodPost, server.refreshSession))
	mux.HandleFunc("/api/v1/auth/logout", method(http.MethodPost, server.logout))
	// Forgot password and reset. Unauthenticated by necessity — the caller cannot sign in,
	// which is the problem — so the only defences are the token itself, the per-account
	// throttle and the per-source cap. See auth.RequestPasswordReset.
	mux.HandleFunc("/api/v1/auth/password/forgot", method(http.MethodPost, server.forgotPassword))
	mux.HandleFunc("/api/v1/auth/password/reset", method(http.MethodPost, server.resetPassword))
	// start and callback are GET because the browser navigates to them; neither
	// changes anything on its own — start only writes a short-lived state, and the
	// callback's effect is gated on that state.
	mux.HandleFunc("/api/v1/auth/oauth/google/start", method(http.MethodGet, server.startGoogleOAuth))
	mux.HandleFunc("/api/v1/auth/oauth/google/callback", method(http.MethodGet, server.googleOAuthCallback))
	// connect is POST and authenticated, and answers with a URL instead of a redirect.
	// See connectGoogleOAuth for why it cannot be a navigation like the other two.
	mux.HandleFunc("/api/v1/auth/oauth/google/connect",
		method(http.MethodPost, server.requireAuth(server.connectGoogleOAuth)))
	// Account settings. Every one acts on the account the bearer token names, so
	// requireAuth is the whole authorization story.
	mux.HandleFunc("/api/v1/account/profile", server.requireAuth(server.accountProfile))
	mux.HandleFunc("/api/v1/account/avatar",
		method(http.MethodPost, server.requireAuth(server.uploadAccountAvatar)))
	// PUT replaces a password the caller proves they know; POST creates the first one for
	// an account that has none. The 11,040 accounts that signed in through Google are the
	// second case, and they have no current password to prove.
	mux.HandleFunc("/api/v1/account/password", server.requireAuth(server.accountPassword))
	// The post editor. Ownership is inside every statement rather than checked before
	// it, so these carry no object id to authorize — a serial that is not the caller's
	// does not resolve at all.
	mux.HandleFunc("/api/v1/account/posts", server.requireAuth(server.accountPosts))
	mux.HandleFunc("/api/v1/account/posts/{serial}", server.requireAuth(server.accountPost))
	mux.HandleFunc("/api/v1/account/posts/{serial}/elements",
		method(http.MethodGet, server.requireAuth(server.listPostElements)))
	mux.HandleFunc("/api/v1/account/elements/{elementID}", server.requireAuth(server.accountElement))
	mux.HandleFunc("/api/v1/account/elements/{elementID}/media",
		method(http.MethodPut, server.requireAuth(server.replaceElementMedia)))
	mux.HandleFunc("/api/v1/account/posts/{serial}/elements/uploads",
		method(http.MethodPost, server.requireAuth(server.uploadPostElement)))
	mux.HandleFunc("/api/v1/account/posts/{serial}/elements/urls",
		method(http.MethodPost, server.requireAuth(server.addPostElementsByURL)))
	// The moderation back office. EVERY ROUTE HERE GOES THROUGH requireAdmin, which is the
	// only thing standing between a signed-in account and somebody else's post: the service
	// underneath acts across owners and does not check the caller. See admin.go.
	mux.HandleFunc("/api/v1/admin/posts", method(http.MethodGet, server.requireAdmin(server.adminPosts)))
	mux.HandleFunc("/api/v1/admin/posts/{serial}", server.requireAdmin(server.adminPost))
	mux.HandleFunc("/api/v1/admin/posts/{serial}/elements",
		method(http.MethodGet, server.requireAdmin(server.adminPostElements)))
	mux.HandleFunc("/api/v1/admin/elements/{elementID}", server.requireAdmin(server.adminElement))
	mux.HandleFunc("/api/v1/admin/users", method(http.MethodGet, server.requireAdmin(server.adminUsers)))
	mux.HandleFunc("/api/v1/admin/users/{userID}/ban", method(http.MethodPut, server.requireAdmin(server.banUser)))
	mux.HandleFunc("/api/v1/admin/users/{userID}/unban", method(http.MethodPut, server.requireAdmin(server.unbanUser)))
	mux.HandleFunc("/api/v1/admin/carousel-items", server.requireAdmin(server.adminCarouselItems))
	mux.HandleFunc("/api/v1/admin/carousel-items/reorder",
		method(http.MethodPut, server.requireAdmin(server.reorderCarouselItems)))
	mux.HandleFunc("/api/v1/admin/carousel-items/{itemID}", server.requireAdmin(server.adminCarouselItem))
	mux.HandleFunc("/api/v1/admin/announcement", server.requireAdmin(server.adminAnnouncement))
	// The bundle's own files. The grant is authenticated like any other admin endpoint;
	// the files themselves are gated by the pass it writes, because a browser cannot put a
	// bearer token on a navigation or a <script src>. See admin_assets.go.
	mux.HandleFunc("/api/v1/admin/assets/grant",
		method(http.MethodPost, server.requireAdmin(server.adminAssetGrant)))
	mux.HandleFunc("/api/v1/admin/assets/revoke",
		method(http.MethodPost, server.requireAdmin(server.adminAssetRevoke)))
	mux.HandleFunc(adminAssetPrefix, server.serveAdminAsset)
	mux.HandleFunc("/api/v1/tags", method(http.MethodGet, server.tags))
	mux.HandleFunc("/api/v1/tags/hot", method(http.MethodGet, server.hotTags))
	mux.HandleFunc("/api/v1/carousel-items", method(http.MethodGet, server.carouselItems))
	mux.HandleFunc("/api/v1/posts", method(http.MethodGet, server.posts))
	mux.HandleFunc("/api/v1/champions", method(http.MethodGet, server.champions))
	// The rank endpoints take optionalAuth for the same reason the game ones do: a post's
	// ranks are as protected as the post, and an owner reads their own without a door code.
	mux.HandleFunc("/api/v1/ranks", method(http.MethodGet, server.optionalAuth(server.ranks)))
	mux.HandleFunc("/api/v1/rank", method(http.MethodGet, server.optionalAuth(server.rank)))
	mux.HandleFunc("/api/v1/rank/search", method(http.MethodGet, server.optionalAuth(server.searchRanks)))
	mux.HandleFunc("/api/v1/posts/{serial}/access", method(http.MethodPost, server.optionalAuth(server.grantPostAccess)))
	// optionalAuth on every game path, because a post's own author may play it without
	// entering the door code — GamePolicy::play let the owner through before checking
	// the token, and these endpoints cannot tell an owner from a stranger without it.
	mux.HandleFunc("/api/v1/game-posts/{serial}", method(http.MethodGet, server.optionalAuth(server.gameDefinition)))
	mux.HandleFunc("/api/v1/games", method(http.MethodPost, server.optionalAuth(server.createGame)))
	mux.HandleFunc("/api/v1/games/{serial}/elements", method(http.MethodGet, server.optionalAuth(server.resumeGame)))
	mux.HandleFunc("/api/v1/games/{serial}/result", method(http.MethodGet, server.optionalAuth(server.gameResult)))
	mux.HandleFunc("/api/v1/games/{serial}/votes/batch", method(http.MethodPost, server.optionalAuth(server.submitGameVotes)))
	// Game rooms. optionalAuth throughout: a room is playable without an account, and
	// identity inside one is the browser's anonymous id — the account is recorded on the
	// participant row but never required.
	mux.HandleFunc("/api/v1/game-rooms", method(http.MethodPost, server.optionalAuth(server.createGameRoom)))
	mux.HandleFunc("/api/v1/game-rooms/{serial}", method(http.MethodGet, server.optionalAuth(server.gameRoomState)))
	// The room follows its host into a new game: what a restart needs, so the invite link
	// already handed out keeps working.
	mux.HandleFunc("/api/v1/game-rooms/{serial}/game",
		method(http.MethodPut, server.optionalAuth(server.rebindGameRoom)))
	mux.HandleFunc("/api/v1/game-rooms/{serial}/leaderboard",
		method(http.MethodGet, server.optionalAuth(server.gameRoomLeaderboard)))
	mux.HandleFunc("/api/v1/game-rooms/{serial}/votes",
		method(http.MethodGet, server.optionalAuth(server.gameRoomVotes)))
	mux.HandleFunc("/api/v1/game-rooms/{serial}/bets", method(http.MethodPost, server.optionalAuth(server.placeGameRoomBet)))
	mux.HandleFunc("/api/v1/game-rooms/{serial}/player", method(http.MethodPut, server.optionalAuth(server.renameGameRoomPlayer)))
	mux.HandleFunc("/api/v1/posts/{serial}/comments", server.optionalAuth(server.commentsCollection))
	mux.HandleFunc("/api/v1/posts/{serial}/comments/{commentID}", method(http.MethodDelete, server.optionalAuth(server.deleteComment)))
	mux.HandleFunc("/api/v1/posts/{serial}/comments/{commentID}/report", method(http.MethodPost, server.optionalAuth(server.reportComment)))
	mux.HandleFunc("/", server.notFound)

	var handler http.Handler = mux
	handler = server.securityHeaders(handler)
	handler = server.cors(handler)
	handler = server.recoverPanic(handler)
	handler = server.accessLog(handler)
	handler = server.requestID(handler)
	return handler
}

func method(allowedMethod string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed := r.Method == allowedMethod || (allowedMethod == http.MethodGet && r.Method == http.MethodHead)
		if !allowed {
			allowHeader := allowedMethod
			if allowedMethod == http.MethodGet {
				allowHeader = http.MethodGet + ", " + http.MethodHead
			}
			w.Header().Set("Allow", allowHeader)
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		handler(w, r)
	}
}

func (a *api) live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, http.StatusOK, envelope{Data: map[string]string{"status": "ok"}})
}

func (a *api) readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !a.ready() {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "service is not ready")
		return
	}
	writeJSON(w, r, http.StatusOK, envelope{Data: map[string]string{"status": "ok"}})
}

func (a *api) systemInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=0")
	w.Header().Set("Cloudflare-CDN-Cache-Control", "public, max-age=60, stale-while-revalidate=30, stale-if-error=3600")
	writeJSON(w, r, http.StatusOK, envelope{Data: map[string]string{
		"service":     a.serviceName,
		"version":     a.version,
		"commit":      a.commit,
		"environment": a.environment,
		"time":        a.now().UTC().Format(time.RFC3339),
	}})
}

func (a *api) currentIdentity(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Cloudflare-CDN-Cache-Control", "no-store")

	response := map[string]any{
		"subject":    identity.Subject,
		"roles":      identity.Roles,
		"expires_at": identity.ExpiresAt.Format(time.RFC3339),
	}

	// The account itself, when this process can read it. Optional so the endpoint keeps
	// working as a pure token check where it always was one — but with it, this replaces
	// the `user` half of Laravel's /session-context, which is the last thing the SPA
	// needed from PHP.
	if a.profiles != nil {
		userID, err := auth.SubjectToUserID(identity.Subject)
		if err != nil {
			// A token whose subject is not a user id verified correctly, which means it
			// was issued for something else entirely.
			a.logger.Warn("identity_subject_is_not_a_user_id", "subject", identity.Subject)
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}
		profile, err := a.profiles.Profile(r.Context(), userID)
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			// The account was deleted while a valid token was still in flight.
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		case err != nil:
			a.logger.Error("profile_lookup_failed", "user_id", userID, "error", err)
			writeError(w, r, http.StatusInternalServerError, "internal_error",
				"the request could not be completed")
			return
		}
		response["user"] = profile
	}

	writeJSON(w, r, http.StatusOK, envelope{Data: response})
}

func (a *api) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
}

func (a *api) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" || len(requestID) > 128 {
			requestID = newRequestID()
		}
		w.Header().Set(requestIDHeader, requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *api) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		a.logger.Info("http_request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

func (a *api) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("http_panic",
					"request_id", requestIDFromContext(r.Context()),
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *api) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (a *api) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.authVerifier == nil {
			writeError(w, r, http.StatusServiceUnavailable, "auth_not_configured", "authentication bridge is not configured")
			return
		}

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "a bearer token is required")
			return
		}

		identity, err := a.authVerifier.Verify(parts[1])
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeError(w, r, http.StatusUnauthorized, "invalid_token", "the bearer token is invalid or expired")
			return
		}

		next(w, r.WithContext(auth.WithIdentity(r.Context(), identity)))
	}
}

func (a *api) optionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) == 0 {
			next(w, r)
			return
		}
		if a.authVerifier == nil {
			writeError(w, r, http.StatusServiceUnavailable, "auth_not_configured", "authentication bridge is not configured")
			return
		}
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, r, http.StatusUnauthorized, "unauthenticated", "a bearer token is required")
			return
		}
		identity, err := a.authVerifier.Verify(parts[1])
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeError(w, r, http.StatusUnauthorized, "invalid_token", "the bearer token is invalid or expired")
			return
		}
		next(w, r.WithContext(auth.WithIdentity(r.Context(), identity)))
	}
}

func (a *api) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Add("Vary", "Origin")
		if !a.originAllowed(origin, r.Host) {
			writeError(w, r, http.StatusForbidden, "origin_not_allowed", "origin not allowed")
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		// The access header is exposed as well as accepted: a response carries a
		// refreshed token, and a browser cannot read a header it was not offered.
		w.Header().Set("Access-Control-Expose-Headers", requestIDHeader+", "+postAccessHeader)
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-TOKEN, X-Requested-With, X-Request-ID, "+postAccessHeader)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *api) originAllowed(origin, requestHost string) bool {
	if _, exists := a.allowedOrigins[origin]; exists {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, requestHost)
}

// clientIP is the address the request came from, as against the address it last
// hopped through. This api never faces a browser directly: nginx in the frontend
// container proxies /api/ to it, with Cloudflare in front of that in production, so
// RemoteAddr is a proxy and is the same value for every visitor on earth. Anything
// that counts per source, or records who acted, has to read the left-most
// X-Forwarded-For entry instead — nginx appends the peer to it on every hop.
//
// THE HEADER IS CLIENT-SUPPLIED, so this value must never authorise anything. A
// caller can send its own X-Forwarded-For and get a key of its choosing, which is
// why the password reset limit keyed on it is a flood guard and not a defence: the
// limit that cannot be sidestepped is the per-account throttle. Reading RemoteAddr
// instead would not be safer, only useless — one shared key would rate-limit every
// user of the site at once.
//
// The result is validated as an address because the header is arbitrary text and
// the audit columns it lands in are VARCHAR(45). Unparseable input yields "",
// which callers treat as "unknown" rather than storing a fragment of a header.
func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if address, err := netip.ParseAddr(forwarded); err == nil {
		return address.Unmap().String()
	}
	host := r.RemoteAddr
	if split, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = split
	}
	if address, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
		return address.Unmap().String()
	}
	return ""
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload envelope) {
	if payload.Meta == nil {
		payload.Meta = &meta{RequestID: requestIDFromContext(r.Context())}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, status, envelope{Error: &apiErr{Code: code, Message: message}})
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	return requestID
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(payload []byte) (int, error) {
	written, err := r.ResponseWriter.Write(payload)
	r.bytes += written
	return written, err
}

// httpWriter is only used by the fallback logger in tests and small tools.
type httpWriter struct{}

func (httpWriter) Write(payload []byte) (int, error) { return len(payload), nil }
