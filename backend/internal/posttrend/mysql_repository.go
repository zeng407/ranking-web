package posttrend

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MySQLRepository implements Repository.
type MySQLRepository struct {
	database *sql.DB
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database}
}

const dateLayout = "2006-01-02"

// playCountsQuery counts qualifying games per post in one pass.
//
// The PHP used Post::withCount(...)->eachById(...), which is one COUNT subquery per
// post plus an upsert per post — around 12,400 round trips for 6,200 posts. This is
// one GROUP BY over games instead.
//
// LEFT JOIN, not JOIN: eachById iterates every post, so a post with no qualifying
// games still gets a statistics row with a count of zero. An inner join would
// silently drop those, and a post that stopped being played would keep its old count
// forever.
//
// posts.deleted_at IS NULL comes from the Post model's SoftDeletes, which Eloquent
// applies to withCount automatically.
const playCountsQuery = `
	SELECT p.id, p.created_at, COUNT(g.id) AS play_count
	  FROM posts AS p
	  LEFT JOIN games AS g
	    ON g.post_id = p.id
	   AND g.vote_count >= ?
	   %s
	 WHERE p.deleted_at IS NULL
	 GROUP BY p.id, p.created_at`

func (repository *MySQLRepository) PlayCounts(
	ctx context.Context, rangeValue TimeRange, windowStart *time.Time,
) ([]PlayCount, error) {
	arguments := []any{MinimumVoteCount}
	windowClause := ""
	if windowStart != nil {
		// Inside the join condition rather than the WHERE clause: on the outer side of
		// a LEFT JOIN it would turn the join into an inner one and drop every post with
		// no games in the window.
		windowClause = "AND g.created_at >= ?"
		arguments = append(arguments, windowStart.Format(dateLayout))
	}

	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(playCountsQuery, windowClause), arguments...)
	if err != nil {
		return nil, fmt.Errorf("posttrend: count plays for %q: %w", rangeValue, err)
	}
	defer rows.Close()

	counts := make([]PlayCount, 0, 8192)
	for rows.Next() {
		var (
			postID    int64
			createdAt sql.NullTime
			count     int64
		)
		if err := rows.Scan(&postID, &createdAt, &count); err != nil {
			return nil, fmt.Errorf("posttrend: scan play count: %w", err)
		}

		entry := PlayCount{PostID: postID, Count: count}
		if windowStart != nil {
			entry.StartDate = *windowStart
		} else {
			// RangeAll keys each row by the post's own creation date. A post with no
			// created_at cannot be keyed at all, so it is skipped rather than written
			// under a zero date that would collide with every other such post.
			if !createdAt.Valid {
				continue
			}
			entry.StartDate = createdAt.Time
		}
		counts = append(counts, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("posttrend: read play counts: %w", err)
	}
	return counts, nil
}

// upsertPlayCounts relies on post_statistics_unique_index
// (post_id, start_date, time_range), which the table already has. The PHP caught
// MySQL 1062 here and logged a warning, which is the same race this statement makes
// impossible.
func (repository *MySQLRepository) UpsertPlayCounts(
	ctx context.Context, rangeValue TimeRange, counts []PlayCount,
) (int64, error) {
	if len(counts) == 0 {
		return 0, nil
	}

	var written int64
	for start := 0; start < len(counts); start += UpsertChunkSize {
		end := start + UpsertChunkSize
		if end > len(counts) {
			end = len(counts)
		}
		chunk := counts[start:end]

		placeholders := make([]string, 0, len(chunk))
		arguments := make([]any, 0, len(chunk)*4)
		for _, entry := range chunk {
			placeholders = append(placeholders, "(?, ?, ?, ?, NOW(), NOW())")
			arguments = append(arguments,
				entry.PostID, entry.StartDate.Format(dateLayout), string(rangeValue), entry.Count)
		}

		statement := `
			INSERT INTO post_statistics (post_id, start_date, time_range, play_count, created_at, updated_at)
			VALUES ` + strings.Join(placeholders, ", ") + `
			ON DUPLICATE KEY UPDATE
				play_count = VALUES(play_count),
				updated_at = NOW()`

		result, err := repository.database.ExecContext(ctx, statement, arguments...)
		if err != nil {
			return written, fmt.Errorf("posttrend: upsert %d play counts for %q: %w", len(chunk), rangeValue, err)
		}
		affected, _ := result.RowsAffected()
		written += affected
	}
	return written, nil
}

// resetPositionsStatement matches the group on start_date with <=> so the NULL that
// RangeAll stores is matched rather than compared away.
const resetPositionsStatement = `
	UPDATE post_trends
	   SET position = ?, updated_at = NOW()
	 WHERE trend_type = ?
	   AND time_range = ?
	   AND start_date <=> ?`

func (repository *MySQLRepository) ResetPositions(
	ctx context.Context, rangeValue TimeRange, windowStart *time.Time,
) (int64, error) {
	result, err := repository.database.ExecContext(ctx, resetPositionsStatement,
		UnrankedPosition, TrendTypeHot, string(rangeValue), nullableDate(windowStart))
	if err != nil {
		return 0, fmt.Errorf("posttrend: reset positions for %q: %w", rangeValue, err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// rankedPostsQuery is the ordering from UpdatePostTrendsPosition, preserved exactly:
// play_count descending, then posts.id descending, so the newer post wins a tie.
//
// It reads posts without a deleted_at filter, which is not an oversight — the PHP
// used DB::table('posts') and so bypassed the soft-delete scope. Adding the filter
// here would change which posts appear in the trend, and the public post refresh
// already excludes deleted posts downstream.
const rankedPostsQuery = `
	SELECT p.id
	  FROM posts AS p
	  JOIN post_statistics AS ps ON ps.post_id = p.id
	 WHERE ps.time_range = ?
	   %s
	 ORDER BY ps.play_count DESC, p.id DESC
	 LIMIT ?`

func (repository *MySQLRepository) RankedPosts(
	ctx context.Context, rangeValue TimeRange, windowStart *time.Time, limit int,
) ([]int64, error) {
	arguments := []any{string(rangeValue)}
	dateClause := ""
	if windowStart != nil {
		dateClause = "AND ps.start_date = ?"
		arguments = append(arguments, windowStart.Format(dateLayout))
	}
	arguments = append(arguments, limit)

	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(rankedPostsQuery, dateClause), arguments...)
	if err != nil {
		return nil, fmt.Errorf("posttrend: read ranked posts for %q: %w", rangeValue, err)
	}
	defer rows.Close()

	postIDs := make([]int64, 0, limit)
	for rows.Next() {
		var postID int64
		if err := rows.Scan(&postID); err != nil {
			return nil, fmt.Errorf("posttrend: scan ranked post: %w", err)
		}
		postIDs = append(postIDs, postID)
	}
	return postIDs, rows.Err()
}

// UpsertPositions writes the ranked rows.
//
// Requires post_trends_post_type_range_date_unique, added in migration 00005. Before
// that index existed this was a SELECT-then-write and the race it allowed had already
// produced 36 duplicate groups, each with one row stuck at the unranked sentinel.
func (repository *MySQLRepository) UpsertPositions(
	ctx context.Context, rangeValue TimeRange, windowStart *time.Time, positions []TrendPosition,
) (int64, error) {
	if len(positions) == 0 {
		return 0, nil
	}

	startDate := nullableDate(windowStart)
	var written int64
	for start := 0; start < len(positions); start += UpsertChunkSize {
		end := start + UpsertChunkSize
		if end > len(positions) {
			end = len(positions)
		}
		chunk := positions[start:end]

		placeholders := make([]string, 0, len(chunk))
		arguments := make([]any, 0, len(chunk)*5)
		for _, entry := range chunk {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, NOW(), NOW())")
			arguments = append(arguments,
				entry.PostID, TrendTypeHot, string(rangeValue), startDate, entry.Position)
		}

		statement := `
			INSERT INTO post_trends (post_id, trend_type, time_range, start_date, position, created_at, updated_at)
			VALUES ` + strings.Join(placeholders, ", ") + `
			ON DUPLICATE KEY UPDATE
				position = VALUES(position),
				updated_at = NOW()`

		result, err := repository.database.ExecContext(ctx, statement, arguments...)
		if err != nil {
			return written, fmt.Errorf("posttrend: upsert %d positions for %q: %w", len(chunk), rangeValue, err)
		}
		affected, _ := result.RowsAffected()
		written += affected
	}
	return written, nil
}

// nullableDate turns the optional window start into a driver value, keeping NULL
// distinct from a zero date.
func nullableDate(windowStart *time.Time) any {
	if windowStart == nil {
		return nil
	}
	return windowStart.Format(dateLayout)
}
