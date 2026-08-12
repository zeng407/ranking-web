package sitemap

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLRepository implements Repository.
type MySQLRepository struct {
	database *sql.DB
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database}
}

// recentPublicPostsQuery mirrors the original's PublicPost query, joined to posts
// for the serial the short URLs use.
//
// Soft-deleted posts are excluded. The original relies on the Post model's scope for
// that, and listing a deleted post would put a 404 in the sitemap.
//
// Cursor paging on public_posts.id is what eachById does.
const recentPublicPostsQuery = `
	SELECT pp.id, p.serial, COALESCE(pp.updated_at, pp.created_at)
	  FROM public_posts pp
	  JOIN posts p ON p.id = pp.post_id AND p.deleted_at IS NULL
	 WHERE pp.created_at >= ? AND pp.id > ?
	 ORDER BY pp.id
	 LIMIT ?`

func (repository *MySQLRepository) RecentPublicPostSerials(
	ctx context.Context, since time.Time, afterID int64, limit int,
) ([]PublicPost, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("sitemap: page size must be positive, got %d", limit)
	}

	rows, err := repository.database.QueryContext(ctx, recentPublicPostsQuery,
		since.Format("2006-01-02 15:04:05"), afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("sitemap: query public posts: %w", err)
	}
	defer rows.Close()

	posts := make([]PublicPost, 0, limit)
	for rows.Next() {
		var (
			post      PublicPost
			serial    sql.NullString
			updatedAt sql.NullTime
		)
		if err := rows.Scan(&post.PublicPostID, &serial, &updatedAt); err != nil {
			return nil, fmt.Errorf("sitemap: scan public post: %w", err)
		}
		if serial.Valid {
			post.PostSerial = serial.String
		}
		if updatedAt.Valid {
			post.UpdatedAt = updatedAt.Time
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sitemap: iterate public posts: %w", err)
	}
	return posts, nil
}
