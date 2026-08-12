package publicpost

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FallbackCandidateLimit bounds how many elements the fallback preview draws from.
//
// The PHP loaded every element of the post and shuffled the lot. That is unbounded —
// a post can hold hundreds of elements — and the fallback only fires for a post with
// fewer than two rank reports, which in practice means one nobody has played yet. So
// the candidates are the first this many by id and two are drawn from those. The
// choice was already arbitrary, and this keeps a chunk's memory predictable.
const FallbackCandidateLimit = 50

// MySQLRepository implements Repository.
type MySQLRepository struct {
	database *sql.DB
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database}
}

const dateLayout = "2006-01-02"

// accessPolicyPublic and accessPolicyPrivate are the post_policies enum values.
const (
	accessPolicyPublic  = "public"
	accessPolicyPrivate = "private"
)

// listedPostIDsQuery is the PassNew source set.
//
// The element requirement is a join and a HAVING rather than the correlated subquery
// whereHas would generate, so it costs one pass instead of one COUNT per post. The
// join to elements carries deleted_at IS NULL because Element uses SoftDeletes, which
// Eloquent applies to the relation automatically — without it a post whose elements
// were all deleted would stay listed.
//
// NO LIMIT. The PHP capped this at 2,000, which meant only the newest 2,000 public
// posts ever reached the listing table. See the package comment.
const listedPostIDsQuery = `
	SELECT p.id
	  FROM posts AS p
	  JOIN post_policies AS pol ON pol.post_id = p.id AND pol.access_policy = ?
	  JOIN post_elements AS pe ON pe.post_id = p.id
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE p.deleted_at IS NULL
	 GROUP BY p.id
	HAVING COUNT(*) >= ?
	 ORDER BY p.id DESC`

func (repository *MySQLRepository) ListedPostIDs(ctx context.Context) ([]int64, error) {
	rows, err := repository.database.QueryContext(ctx, listedPostIDsQuery,
		accessPolicyPublic, MinimumElementCount)
	if err != nil {
		return nil, fmt.Errorf("publicpost: list post ids: %w", err)
	}
	return scanIDs(rows, "publicpost: scan listed post id")
}

// trendedPostIDsQuery is the source set for the day, week and month passes: the hot
// trend for one window, restricted to posts that still qualify for the listing.
const trendedPostIDsQuery = `
	SELECT pt.post_id
	  FROM post_trends AS pt
	  JOIN posts AS p ON p.id = pt.post_id AND p.deleted_at IS NULL
	  JOIN post_policies AS pol ON pol.post_id = p.id AND pol.access_policy = ?
	  JOIN post_elements AS pe ON pe.post_id = p.id
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE pt.trend_type = 'hot'
	   AND pt.time_range = ?
	   AND pt.start_date = ?
	 GROUP BY pt.post_id, pt.position
	HAVING COUNT(*) >= ?
	 ORDER BY pt.position ASC`

func (repository *MySQLRepository) TrendedPostIDs(
	ctx context.Context, trendRange string, windowStart time.Time,
) ([]int64, error) {
	rows, err := repository.database.QueryContext(ctx, trendedPostIDsQuery,
		accessPolicyPublic, trendRange, windowStart.Format(dateLayout), MinimumElementCount)
	if err != nil {
		return nil, fmt.Errorf("publicpost: list %q trend post ids: %w", trendRange, err)
	}
	return scanIDs(rows, "publicpost: scan trended post id")
}

func scanIDs(rows *sql.Rows, message string) ([]int64, error) {
	defer rows.Close()
	ids := make([]int64, 0, 4096)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: %w", message, err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MarkAllDirty flags every row. No updated_at bump: the PHP used
// PublicPost::getQuery(), which bypasses Eloquent's timestamps, and the flag is
// bookkeeping rather than a change to the listing.
func (repository *MySQLRepository) MarkAllDirty(ctx context.Context) (int64, error) {
	result, err := repository.database.ExecContext(ctx,
		"UPDATE public_posts SET is_dirty = 1 WHERE is_dirty = 0")
	if err != nil {
		return 0, fmt.Errorf("publicpost: mark rows dirty: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// PushDirtyToSentinel gives every row this pass did not write the unlisted position.
//
// This is what removes a post from a listing: it gets no upsert, stays dirty, and
// lands on 9999 which sorts last.
func (repository *MySQLRepository) PushDirtyToSentinel(ctx context.Context, pass Pass) (int64, error) {
	column, err := pass.PositionColumn()
	if err != nil {
		return 0, err
	}
	result, err := repository.database.ExecContext(ctx,
		"UPDATE public_posts SET "+column+" = ? WHERE is_dirty = 1", UnlistedPosition)
	if err != nil {
		return 0, fmt.Errorf("publicpost: push dirty rows to the sentinel for %q: %w", pass, err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// LoadChunk assembles the rows for one chunk of post ids.
//
// Six batched queries rather than the PHP's several per post. The positions are
// assigned by the caller, which is the only thing that needs the whole ordered list.
func (repository *MySQLRepository) LoadChunk(ctx context.Context, postIDs []int64) ([]Row, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}

	posts, err := repository.postRows(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	tags, err := repository.tagsByPost(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	elementCounts, err := repository.elementCounts(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	playCounts, err := repository.playCounts(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	ranked, err := repository.rankedElements(ctx, postIDs)
	if err != nil {
		return nil, err
	}

	// Only the posts the ranked set cannot supply two elements for need the fallback,
	// so the unbounded query is asked for as little as possible.
	needy := make([]int64, 0, len(postIDs))
	for _, postID := range postIDs {
		if len(ranked[postID]) < 2 {
			needy = append(needy, postID)
		}
	}
	fallback, err := repository.fallbackElements(ctx, needy)
	if err != nil {
		return nil, err
	}

	assembled := make([]Row, 0, len(postIDs))
	for _, postID := range postIDs {
		post, ok := posts[postID]
		if !ok {
			// Deleted between the id list and this query. Skipping leaves it dirty, so
			// RemoveDirty deals with it.
			continue
		}
		assembled = append(assembled, Row{
			PostID:      postID,
			Title:       post.Title,
			Description: post.Description,
			Resource: BuildResource(post, tags[postID], elementCounts[postID], playCounts[postID],
				nil, nil),
			// Tags is filled by the caller together with the preview selection, which
			// needs the injected shuffle.
			Tags: "",
		})
		// Carry the candidate sets on the row so the caller can select without a
		// second round trip.
		assembled[len(assembled)-1].rankedCandidates = ranked[postID]
		assembled[len(assembled)-1].fallbackCandidates = fallback[postID]
		assembled[len(assembled)-1].tagNames = tags[postID]
	}
	return assembled, nil
}

const postRowsQuery = `
	SELECT p.id, p.serial, COALESCE(p.title, ''), COALESCE(p.description, ''),
	       p.is_censored, p.created_at, p.updated_at,
	       COALESCE(pol.access_policy, '')
	  FROM posts AS p
	  LEFT JOIN post_policies AS pol ON pol.post_id = p.id
	 WHERE p.deleted_at IS NULL AND p.id IN (%s)`

func (repository *MySQLRepository) postRows(ctx context.Context, postIDs []int64) (map[int64]PostRow, error) {
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(postRowsQuery, placeholders(len(postIDs))), anyIDs(postIDs)...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: read posts: %w", err)
	}
	defer rows.Close()

	byID := make(map[int64]PostRow, len(postIDs))
	for rows.Next() {
		var (
			post         PostRow
			accessPolicy string
			createdAt    sql.NullTime
			updatedAt    sql.NullTime
		)
		if err := rows.Scan(&post.ID, &post.Serial, &post.Title, &post.Description,
			&post.IsCensored, &createdAt, &updatedAt, &accessPolicy); err != nil {
			return nil, fmt.Errorf("publicpost: scan post: %w", err)
		}
		post.IsPrivate = accessPolicy == accessPolicyPrivate
		if createdAt.Valid {
			post.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			post.UpdatedAt = updatedAt.Time
		}
		byID[post.ID] = post
	}
	return byID, rows.Err()
}

// tagsByPost reads the pivot in a stable order. The Eloquent relation left the order
// to the database; ordering by tag id here makes the stored payload reproducible.
const tagsByPostQuery = `
	SELECT pt.post_id, t.name
	  FROM post_tags AS pt
	  JOIN tags AS t ON t.id = pt.tag_id
	 WHERE pt.post_id IN (%s)
	 ORDER BY pt.post_id, t.id`

func (repository *MySQLRepository) tagsByPost(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(tagsByPostQuery, placeholders(len(postIDs))), anyIDs(postIDs)...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: read tags: %w", err)
	}
	defer rows.Close()

	byPost := make(map[int64][]string, len(postIDs))
	for rows.Next() {
		var (
			postID int64
			name   string
		)
		if err := rows.Scan(&postID, &name); err != nil {
			return nil, fmt.Errorf("publicpost: scan tag: %w", err)
		}
		byPost[postID] = append(byPost[postID], name)
	}
	return byPost, rows.Err()
}

const elementCountsQuery = `
	SELECT pe.post_id, COUNT(*)
	  FROM post_elements AS pe
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE pe.post_id IN (%s)
	 GROUP BY pe.post_id`

func (repository *MySQLRepository) elementCounts(ctx context.Context, postIDs []int64) (map[int64]int64, error) {
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(elementCountsQuery, placeholders(len(postIDs))), anyIDs(postIDs)...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: count elements: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int64, len(postIDs))
	for rows.Next() {
		var (
			postID int64
			count  int64
		)
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, fmt.Errorf("publicpost: scan element count: %w", err)
		}
		counts[postID] = count
	}
	return counts, rows.Err()
}

// playCounts is Post::getAllPlayedCount: the all-time statistics row, or zero.
//
// PostResource writes `getAllPlayedCount() ?? $this->games()->count()`, but
// getAllPlayedCount already ends in `?? 0` and so never returns null — the games()
// fallback is unreachable. Only the statistics value is used here.
const playCountsQuery = `
	SELECT post_id, play_count
	  FROM post_statistics
	 WHERE time_range = 'all' AND post_id IN (%s)`

func (repository *MySQLRepository) playCounts(ctx context.Context, postIDs []int64) (map[int64]int64, error) {
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(playCountsQuery, placeholders(len(postIDs))), anyIDs(postIDs)...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: read play counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int64, len(postIDs))
	for rows.Next() {
		var (
			postID int64
			count  int64
		)
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, fmt.Errorf("publicpost: scan play count: %w", err)
		}
		counts[postID] = count
	}
	return counts, rows.Err()
}

// rankedElementsQuery is the top candidates the preview prefers.
//
// The ordering mirrors CacheService::rememberRankReports:
// `orderByRaw('CASE WHEN rank IS NULL THEN 1 ELSE 0 END')` then `orderBy('rank')`, so
// unranked reports sort after ranked ones. rr.id breaks the remaining ties, which the
// PHP left to the database.
const rankedElementsQuery = `
	SELECT ranked.post_id, e.id, e.title, e.type, e.video_source, e.thumb_url, e.mediumthumb_url
	  FROM (
		SELECT rr.post_id, rr.element_id,
		       ROW_NUMBER() OVER (
		           PARTITION BY rr.post_id
		           ORDER BY (rr.rank IS NULL) ASC, rr.rank ASC, rr.id ASC
		       ) AS candidate
		  FROM rank_reports AS rr
		  JOIN elements AS ranked_element
		    ON ranked_element.id = rr.element_id AND ranked_element.deleted_at IS NULL
		 WHERE rr.post_id IN (%s)
	  ) AS ranked
	  JOIN elements AS e ON e.id = ranked.element_id
	 WHERE ranked.candidate <= ?
	 ORDER BY ranked.post_id, ranked.candidate`

func (repository *MySQLRepository) rankedElements(ctx context.Context, postIDs []int64) (map[int64][]ElementRow, error) {
	arguments := append(anyIDs(postIDs), PreviewCandidateLimit)
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(rankedElementsQuery, placeholders(len(postIDs))), arguments...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: read ranked elements: %w", err)
	}
	return scanElementsByPost(rows)
}

// fallbackElementsQuery supplies previews for a post with fewer than two rank
// reports. Bounded by FallbackCandidateLimit; see that constant.
const fallbackElementsQuery = `
	SELECT candidates.post_id, e.id, e.title, e.type, e.video_source, e.thumb_url, e.mediumthumb_url
	  FROM (
		SELECT pe.post_id, pe.element_id,
		       ROW_NUMBER() OVER (PARTITION BY pe.post_id ORDER BY pe.element_id ASC) AS candidate
		  FROM post_elements AS pe
		  JOIN elements AS listed ON listed.id = pe.element_id AND listed.deleted_at IS NULL
		 WHERE pe.post_id IN (%s)
	  ) AS candidates
	  JOIN elements AS e ON e.id = candidates.element_id
	 WHERE candidates.candidate <= ?
	 ORDER BY candidates.post_id, candidates.candidate`

func (repository *MySQLRepository) fallbackElements(ctx context.Context, postIDs []int64) (map[int64][]ElementRow, error) {
	if len(postIDs) == 0 {
		return map[int64][]ElementRow{}, nil
	}
	arguments := append(anyIDs(postIDs), FallbackCandidateLimit)
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(fallbackElementsQuery, placeholders(len(postIDs))), arguments...)
	if err != nil {
		return nil, fmt.Errorf("publicpost: read fallback elements: %w", err)
	}
	return scanElementsByPost(rows)
}

func scanElementsByPost(rows *sql.Rows) (map[int64][]ElementRow, error) {
	defer rows.Close()

	byPost := make(map[int64][]ElementRow)
	for rows.Next() {
		var (
			postID  int64
			element ElementRow
		)
		if err := rows.Scan(&postID, &element.ID, &element.Title, &element.Type,
			&element.VideoSource, &element.ThumbURL, &element.MediumThumbURL); err != nil {
			return nil, fmt.Errorf("publicpost: scan element: %w", err)
		}
		byPost[postID] = append(byPost[postID], element)
	}
	return byPost, rows.Err()
}

// UpsertChunk writes one chunk.
//
// Only this pass's position column is touched, matching the PHP's per-pass
// updateOrCreate: the other three keep whatever the other passes wrote, and on insert
// they take the column default of 9999.
//
// Requires public_posts_post_id_unique from migration 00007. Before that index, this
// was a SELECT-then-write and the race had already produced 3 duplicate listings.
func (repository *MySQLRepository) UpsertChunk(ctx context.Context, pass Pass, rows []Row) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	column, err := pass.PositionColumn()
	if err != nil {
		return 0, err
	}

	placeholderRows := make([]string, 0, len(rows))
	arguments := make([]any, 0, len(rows)*6)
	for _, row := range rows {
		payload, err := json.Marshal(row.Resource)
		if err != nil {
			return 0, fmt.Errorf("publicpost: encode resource for post %d: %w", row.PostID, err)
		}
		placeholderRows = append(placeholderRows, "(?, ?, ?, ?, ?, ?, 0, NOW(), NOW())")
		arguments = append(arguments,
			row.PostID, row.Position, row.Title, row.Description, row.Tags, payload)
	}

	statement := `
		INSERT INTO public_posts
		       (post_id, ` + column + `, title, description, tags, data, is_dirty, created_at, updated_at)
		VALUES ` + strings.Join(placeholderRows, ", ") + `
		ON DUPLICATE KEY UPDATE
			` + column + ` = VALUES(` + column + `),
			title = VALUES(title),
			description = VALUES(description),
			tags = VALUES(tags),
			data = VALUES(data),
			is_dirty = 0,
			updated_at = NOW()`

	result, err := repository.database.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return 0, fmt.Errorf("publicpost: upsert %d rows for %q: %w", len(rows), pass, err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// removeDirtyStatement deletes listings whose post no longer qualifies.
//
// The three conditions are the three branches of removeDirtyPublicPosts: the post is
// gone (or soft deleted, which is what made `$publicPost->post` null), it is no longer
// public, or it has dropped below the element minimum.
//
// Set-based rather than a row-at-a-time loop, and with no limit: the PHP capped it at
// 2,000, which left the rest of the stale listings in place until some later run
// happened to reach them.
const removeDirtyStatement = `
	DELETE pp
	  FROM public_posts AS pp
	  LEFT JOIN posts AS p ON p.id = pp.post_id AND p.deleted_at IS NULL
	  LEFT JOIN post_policies AS pol ON pol.post_id = p.id AND pol.access_policy = ?
	  LEFT JOIN (
		SELECT pe.post_id, COUNT(*) AS element_count
		  FROM post_elements AS pe
		  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
		 GROUP BY pe.post_id
	  ) AS counted ON counted.post_id = pp.post_id
	 WHERE pp.is_dirty = 1
	   AND (p.id IS NULL OR pol.post_id IS NULL OR COALESCE(counted.element_count, 0) < ?)`

func (repository *MySQLRepository) RemoveDirty(ctx context.Context) (int64, error) {
	result, err := repository.database.ExecContext(ctx, removeDirtyStatement,
		accessPolicyPublic, MinimumElementCount)
	if err != nil {
		return 0, fmt.Errorf("publicpost: remove stale listings: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (repository *MySQLRepository) PublicPostIDs(ctx context.Context) ([]int64, error) {
	rows, err := repository.database.QueryContext(ctx, "SELECT post_id FROM public_posts")
	if err != nil {
		return nil, fmt.Errorf("publicpost: list public post ids: %w", err)
	}
	return scanIDs(rows, "publicpost: scan public post id")
}

func placeholders(count int) string {
	if count == 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?, ", count), ", ")
}

func anyIDs(ids []int64) []any {
	arguments := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		arguments = append(arguments, id)
	}
	return arguments
}
