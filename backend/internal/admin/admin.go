// Package admin is the moderation back office, from Laravel's Admin\* controllers and
// routes/admin-api.php.
//
// Every call here acts on rows that belong to somebody else, which is the whole point of
// the package and the reason it is separate from internal/authoring: there, ownership
// rides along inside every statement, so no endpoint can forget it. A moderator has no
// ownership to check against, so the authorization has to live at the HTTP boundary
// instead — httpapi.requireAdmin — and this package is written to be reachable only from
// behind it. Nothing in here re-checks the caller's role, and nothing in here should be
// wired to a route that does not.
//
// Where a moderator does the same thing an author does — edit a post, retitle an
// element, delete either — the work is delegated to internal/authoring against the
// owner's id rather than reimplemented. That keeps one copy of the rules that matter
// (the password policy on update, the cascade on delete, the rank reports a deletion
// invalidates) instead of a second copy that drifts.
package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"2pick.app/backend/internal/authoring"
)

// Page sizes, from the paginate(10) calls in the Laravel admin controllers.
const (
	PostsPerPage = 10
	UsersPerPage = 10
)

// BannedRoleSlug is App\Enums\Role::BANNED, the role a ban adds and an unban removes.
const BannedRoleSlug = "banned"

// AdminRoleSlug is App\Enums\Role::ADMIN. It is not used to authorize anything in this
// package — see the package comment — but the ban path needs it to refuse a request that
// would lock the moderators out.
const AdminRoleSlug = "admin"

// ErrNotFound means the post, element, user or carousel item does not exist.
//
// Unlike the authoring package, this one has nothing to hide: the caller is already
// known to be a moderator, so 404 here means what it says.
var ErrNotFound = errors.New("admin: not found")

// ErrCannotBanAdministrator refuses to ban an account that holds the admin role.
//
// The original had no such rule, so one moderator could ban another — or themselves —
// and the only way back was a database edit. A ban revokes every session, which makes
// self-banning immediate and unrecoverable through the UI.
var ErrCannotBanAdministrator = errors.New("admin: administrators cannot be banned")

// Validation reuses the authoring package's codes and error type so one client-side
// renderer covers both forms.
type (
	FieldErrors = authoring.FieldErrors
	ErrInvalid  = authoring.ErrInvalid
)

func invalid(field, code string) error {
	return &authoring.ErrInvalid{Fields: authoring.FieldErrors{field: []string{code}}}
}

// Authoring is the slice of authoring.Service this package delegates to.
//
// Each call takes the owner's user id, which this package resolves from the row itself:
// the statements underneath still carry `AND user_id = ?`, so a moderator's edit is
// exactly the edit the owner could have made. It also means a serial that stopped
// existing between the lookup and the write fails closed with ErrPostNotFound rather
// than writing to something else.
type Authoring interface {
	Post(ctx context.Context, userID int64, serial string) (authoring.Post, error)
	UpdatePost(ctx context.Context, userID int64, serial string, draft authoring.PostDraft) (authoring.Post, error)
	DeletePostAsModerator(ctx context.Context, userID int64, serial string) error
	Elements(ctx context.Context, userID int64, serial string, query authoring.ElementQuery) (authoring.ElementPage, error)
	EditElement(ctx context.Context, userID int64, elementID int64, edit authoring.ElementEdit) (authoring.Element, error)
	DeleteElement(ctx context.Context, userID int64, elementID int64) error
}

// Store is the persistence this package needs on top of the authoring service: the
// cross-owner reads a moderator makes, the censorship flag, the role pivot and the home
// carousel.
type Store interface {
	// PostOwner answers the user id behind a serial, and ErrNotFound when there is none.
	PostOwner(ctx context.Context, serial string) (int64, error)
	// ElementOwner answers the user id that owns the post the element belongs to. An
	// element shared by two posts — the schema allows it through the pivot — resolves to
	// the owner of the lowest post id, which is the row the join would have matched.
	ElementOwner(ctx context.Context, elementID int64) (int64, error)
	// ListPosts reads one page of every post on the site, newest first.
	ListPosts(ctx context.Context, page, perPage int) ([]Post, int, error)
	// SetPostCensored writes posts.is_censored.
	SetPostCensored(ctx context.Context, serial string, censored bool) error
	// ListUsers reads one page of accounts. A non-empty keyword matches the name or the
	// address; the empty keyword matches everything.
	ListUsers(ctx context.Context, keyword string, page, perPage int) ([]User, int, error)
	// UserRoles reads an account's role slugs, and ErrNotFound when the account is gone.
	UserRoles(ctx context.Context, userID int64) ([]string, error)
	// AddRole attaches a role by slug, and is a no-op when the account already holds it.
	AddRole(ctx context.Context, userID int64, slug string) error
	// RemoveRole detaches a role by slug, and is a no-op when the account does not hold it.
	RemoveRole(ctx context.Context, userID int64, slug string) error
	// CarouselItems reads the home carousel in the order the admin screen lists it.
	CarouselItems(ctx context.Context) ([]CarouselItem, error)
	// CarouselItem reads one item, and ErrNotFound when it is gone.
	CarouselItem(ctx context.Context, itemID int64) (CarouselItem, error)
	// CreateCarouselItem inserts an item and answers with the stored row.
	CreateCarouselItem(ctx context.Context, item CarouselItem) (CarouselItem, error)
	// UpdateCarouselItem writes the fields the edit names and answers with the stored row.
	UpdateCarouselItem(ctx context.Context, itemID int64, edit CarouselEdit) (CarouselItem, error)
	// DeleteCarouselItem soft-deletes an item.
	DeleteCarouselItem(ctx context.Context, itemID int64) error
	// ReorderCarouselItems writes the given positions in one transaction, so a failure
	// halfway cannot leave the carousel in an order nobody asked for.
	ReorderCarouselItems(ctx context.Context, positions []CarouselPosition) error
}

// RoleCache invalidates the cached role list Laravel reads.
//
// Laravel answers isAdmin() and the ban check from CacheService::rememberUserRole, a
// one-hour cache keyed user_role_<id>. Go reads the pivot directly, so a ban applies to
// this API as soon as the next token refresh re-reads the roles; without this, the Blade
// pages would keep treating a banned account as unbanned for up to an hour.
type RoleCache interface {
	ForgetUserRoles(ctx context.Context, userID int64) error
}

// CarouselCache invalidates the cached home carousel Laravel serves.
type CarouselCache interface {
	ForgetCarousels(ctx context.Context) error
}

// SessionRevoker ends every session an account holds.
//
// A BAN THAT LEAVES THE SESSIONS ALIVE IS NOT A BAN FOR THE NEXT FIVE MINUTES. The
// original only cleared the role cache, and the account kept whatever it had; here the
// refresh-token family is revoked as well, so the banned account is signed out at once
// and its next refresh fails.
type SessionRevoker interface {
	RevokeAll(ctx context.Context, userID int64) (int64, error)
}

// AnnouncementStore holds the site-wide announcement.
type AnnouncementStore interface {
	// Announcement reads the current announcement. The second result is false when
	// there is none, which is not an error: the banner is normally absent.
	Announcement(ctx context.Context) (Announcement, bool, error)
	// PutAnnouncement replaces it, and lets it expire after the announcement's own
	// KeepMinutes.
	PutAnnouncement(ctx context.Context, announcement Announcement) error
}

// VideoResolver reads the title, thumbnail and id behind a carousel item's video URL.
type VideoResolver interface {
	Resolve(ctx context.Context, videoURL string) (ResolvedVideo, error)
}

// Service holds the moderation rules.
type Service struct {
	authoring     Authoring
	store         Store
	roleCache     RoleCache
	carouselCache CarouselCache
	sessions      SessionRevoker
	announcements AnnouncementStore
	videos        VideoResolver
	now           func() time.Time
}

type ServiceOptions struct {
	Authoring Authoring
	Store     Store
	// RoleCache is optional. Without it a ban still takes effect here and on the Blade
	// pages within the hour the cache lives, which is worse than immediate but better
	// than refusing the ban.
	RoleCache RoleCache
	// CarouselCache is optional in the same way: the home page keeps the old carousel
	// until its own cache expires.
	CarouselCache CarouselCache
	// Sessions is optional. Without it a banned account keeps its access token until it
	// expires, and its refresh then fails on the re-read roles.
	Sessions SessionRevoker
	// Announcements is optional: without it the announcement endpoints answer 503 and
	// every other admin endpoint is unaffected.
	Announcements AnnouncementStore
	// Videos is optional: without it a carousel item can be created from an explicit
	// image and video URL, but not resolved from a video URL alone.
	Videos VideoResolver
	// Now is the clock, injectable so a test can assert on a published timestamp.
	Now func() time.Time
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Authoring == nil {
		return nil, errors.New("admin: authoring service is required")
	}
	if options.Store == nil {
		return nil, errors.New("admin: store is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		now:           options.Now,
		authoring:     options.Authoring,
		store:         options.Store,
		roleCache:     options.RoleCache,
		carouselCache: options.CarouselCache,
		sessions:      options.Sessions,
		announcements: options.Announcements,
		videos:        options.Videos,
	}, nil
}

// wrap adds the operation to a store failure so a 500's log line says which one broke.
func wrap(operation string, err error) error {
	if err == nil || errors.Is(err, ErrNotFound) {
		return err
	}
	return fmt.Errorf("admin: %s: %w", operation, err)
}
