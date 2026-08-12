package ranking

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLPostRepository implements PostRepository.
type MySQLPostRepository struct {
	database *sql.DB
}

func NewMySQLPostRepository(database *sql.DB) *MySQLPostRepository {
	return &MySQLPostRepository{database: database}
}

// Cursor paging on the primary key, which is what Eloquent's chunkById does. Plain
// OFFSET paging would skip or repeat rows while posts are being inserted, and these
// sweeps run against a live table.
const (
	postsQuery = `
		SELECT id, created_at FROM posts
		 WHERE id > ? AND deleted_at IS NULL
		 ORDER BY id
		 LIMIT ?`

	postsIncludingDeletedQuery = `
		SELECT id, created_at FROM posts
		 WHERE id > ?
		 ORDER BY id
		 LIMIT ?`

	reportsForPostQuery = `
		SELECT rr.id, rr.element_id FROM rank_reports rr
		 WHERE rr.post_id = ?
		 ORDER BY rr.id`
)

func (repository *MySQLPostRepository) Posts(ctx context.Context, afterID int64, limit int) ([]PostRef, error) {
	return repository.queryPosts(ctx, postsQuery, afterID, limit)
}

func (repository *MySQLPostRepository) PostsIncludingDeleted(ctx context.Context, afterID int64, limit int) ([]PostRef, error) {
	return repository.queryPosts(ctx, postsIncludingDeletedQuery, afterID, limit)
}

func (repository *MySQLPostRepository) queryPosts(
	ctx context.Context, query string, afterID int64, limit int,
) ([]PostRef, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("ranking: post page size must be positive, got %d", limit)
	}

	rows, err := repository.database.QueryContext(ctx, query, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("ranking: query posts after %d: %w", afterID, err)
	}
	defer rows.Close()

	posts := make([]PostRef, 0, limit)
	for rows.Next() {
		var (
			post      PostRef
			createdAt sql.NullTime
		)
		if err := rows.Scan(&post.ID, &createdAt); err != nil {
			return nil, fmt.Errorf("ranking: scan post: %w", err)
		}
		if createdAt.Valid {
			post.CreatedAt = createdAt.Time
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate posts: %w", err)
	}
	return posts, nil
}

func (repository *MySQLPostRepository) ReportsForPost(ctx context.Context, postID int64) ([]ReportRef, error) {
	rows, err := repository.database.QueryContext(ctx, reportsForPostQuery, postID)
	if err != nil {
		return nil, fmt.Errorf("ranking: query reports for post %d: %w", postID, err)
	}
	defer rows.Close()

	reports := make([]ReportRef, 0)
	for rows.Next() {
		var report ReportRef
		if err := rows.Scan(&report.ID, &report.ElementID); err != nil {
			return nil, fmt.Errorf("ranking: scan report: %w", err)
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate reports: %w", err)
	}
	return reports, nil
}

// PostCreatedAtOrZero is a helper for callers that only have an id.
func PostCreatedAtOrZero(created sql.NullTime) time.Time {
	if created.Valid {
		return created.Time
	}
	return time.Time{}
}
