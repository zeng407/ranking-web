package comments

import (
	"context"
	"errors"
)

var (
	ErrNotFound  = errors.New("comment resource not found")
	ErrRateLimit = errors.New("comment rate limit exceeded")
)

const (
	MaxContentLength = 200
	MaxReportLength  = 200
)

type Viewer struct {
	UserID      *int64
	AnonymousID string
}

type Comment struct {
	ID        int64    `json:"id"`
	Content   string   `json:"content"`
	CreatedAt string   `json:"created_at"`
	EditedAt  *string  `json:"edited_at"`
	Nickname  string   `json:"nickname"`
	AvatarURL *string  `json:"avatar_url"`
	Champions []string `json:"champions"`
}

type Profile struct {
	Nickname        string   `json:"nickname"`
	AvatarURL       *string  `json:"avatar_url"`
	Champions       []string `json:"champions"`
	IsAuthenticated bool     `json:"is_auth"`
}

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
}
