package comments

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("comment resource not found")
	ErrRateLimit     = errors.New("comment rate limit exceeded")
	ErrInvalidParent = errors.New("comment cannot be replied to")
)

const (
	MaxContentLength = 200
	MaxReportLength  = 200
	// MaxDepth is how far replies may nest: a top-level comment, a reply to it, and a
	// reply to that. Past there a thread indents itself into a column two words wide,
	// so the third level is where the reply affordance stops being offered.
	MaxDepth = 3
)

// Viewer is who is asking, in the two ways a commenter can be known.
type Viewer struct {
	UserID      *int64
	AnonymousID string
	// DeleteHash is the SHA-256 of the guest's delete-key cookie, hex encoded, or
	// empty when the browser carries no such cookie. It is what a signed-out
	// commenter's ownership of a comment is decided by; see comments.delete_hash.
	DeleteHash string
}

type Comment struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Depth    int    `json:"depth"`
	// Floor is the comment's position among the post's top-level comments, counted
	// from the oldest, and is null for a reply — replies live under a floor rather
	// than taking one of their own.
	Floor     *int     `json:"floor"`
	Content   string   `json:"content"`
	CreatedAt string   `json:"created_at"`
	EditedAt  *string  `json:"edited_at"`
	Nickname  string   `json:"nickname"`
	AvatarURL *string  `json:"avatar_url"`
	Champions []string `json:"champions"`
	// Deleted marks a tombstone: the row is still listed so its floor number and the
	// replies underneath it survive, but every trace of who wrote it and what it said
	// has been stripped from the response.
	Deleted   bool `json:"deleted"`
	CanDelete bool `json:"can_delete"`
}

type Profile struct {
	Nickname        string   `json:"nickname"`
	AvatarURL       *string  `json:"avatar_url"`
	Champions       []string `json:"champions"`
	IsAuthenticated bool     `json:"is_auth"`
}

// Page is one page of a post's comments.
//
// The unit of pagination is the floor, not the comment: Items carries the page's
// top-level comments and every reply beneath them, so it is routinely longer than
// PerPage. Total counts the comments a reader can actually read — all depths, tombstones
// excluded — because it is the number shown beside the heading, while TotalPages is
// derived from the floor count, tombstones included.
type Page struct {
	Items      []Comment `json:"items"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"total_pages"`
	Profile    Profile   `json:"profile"`
}

type CreateInput struct {
	Content     string
	Anonymous   bool
	AnonymousID string
	IP          string
	ParentID    *int64
	Viewer      Viewer
}

type ReportInput struct {
	Reason      string
	AnonymousID string
	IP          string
	Viewer      Viewer
}

type Repository interface {
	List(ctx context.Context, postSerial string, page, perPage int, viewer Viewer) (Page, error)
	Create(ctx context.Context, postSerial string, input CreateInput) (Comment, error)
	Report(ctx context.Context, postSerial string, commentID int64, input ReportInput) error
	Delete(ctx context.Context, postSerial string, commentID int64, viewer Viewer) error
}
