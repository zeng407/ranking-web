package authoring

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MySQLRepository implements PostStore, ElementStore and PasswordChecker.
type MySQLRepository struct {
	database *sql.DB
	now      func() time.Time
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database, now: time.Now}
}

// OWNERSHIP IS PART OF EVERY STATEMENT, NOT A CHECK BEFORE IT. The original authorized
// with a policy that ran its own query and then acted on a row it had looked up
// separately; here `AND p.user_id = ?` rides along in the same statement, so there is no
// window between the check and the write and no way to add an endpoint that forgets it.
const postsPageQuery = `
	SELECT p.serial,
	       COALESCE(p.title, ''),
	       COALESCE(p.description, ''),
	       pp.access_policy,
	       pp.password IS NOT NULL AND pp.password <> '',
	       p.created_at,
	       COALESCE(all_time.play_count, 0),
	       COALESCE(this_week.play_count, 0),
	       COALESCE(last_week.play_count, 0)
	  FROM posts AS p
	  JOIN post_policies AS pp ON pp.post_id = p.id
	  LEFT JOIN post_statistics AS all_time
	         ON all_time.post_id = p.id AND all_time.time_range = 'all'
	  LEFT JOIN post_statistics AS this_week
	         ON this_week.post_id = p.id AND this_week.time_range = 'week'
	        AND this_week.start_date = ?
	  LEFT JOIN post_statistics AS last_week
	         ON last_week.post_id = p.id AND last_week.time_range = 'week'
	        AND last_week.start_date = ?
	 WHERE p.user_id = ?
	   AND p.deleted_at IS NULL
	 ORDER BY p.id DESC
	 LIMIT ? OFFSET ?`

const postsCountQuery = `
	SELECT COUNT(*) FROM posts WHERE user_id = ? AND deleted_at IS NULL`

// tagsForPostsQuery reads the tags for a whole page in one round trip rather than one
// query per post, which is what the eager load did.
const tagsForPostsQuery = `
	SELECT p.serial, t.name
	  FROM posts AS p
	  JOIN post_tags AS pt ON pt.post_id = p.id
	  JOIN tags AS t ON t.id = pt.tag_id
	 WHERE p.serial IN (%s)
	 ORDER BY t.id`

func (repository *MySQLRepository) ListPosts(
	ctx context.Context, userID int64, page, perPage int,
) ([]Post, int, error) {
	thisWeek, lastWeek := weekBoundaries(repository.now())

	rows, err := repository.database.QueryContext(ctx, postsPageQuery,
		thisWeek, lastWeek, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("authoring: list posts for %d: %w", userID, err)
	}
	defer rows.Close()

	posts := make([]Post, 0, perPage)
	for rows.Next() {
		var (
			post      Post
			createdAt sql.NullTime
		)
		if err := rows.Scan(&post.Serial, &post.Title, &post.Description,
			&post.AccessPolicy, &post.HasPassword, &createdAt,
			&post.PlayCount, &post.ThisWeekPlayCount, &post.LastWeekPlayCount); err != nil {
			return nil, 0, fmt.Errorf("authoring: scan post: %w", err)
		}
		if createdAt.Valid {
			post.CreatedAt = createdAt.Time
		}
		post.Tags = []string{}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("authoring: list posts for %d: %w", userID, err)
	}

	if err := repository.attachTags(ctx, posts); err != nil {
		return nil, 0, err
	}

	var total int
	if err := repository.database.QueryRowContext(ctx, postsCountQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("authoring: count posts for %d: %w", userID, err)
	}
	return posts, total, nil
}

func (repository *MySQLRepository) attachTags(ctx context.Context, posts []Post) error {
	if len(posts) == 0 {
		return nil
	}
	placeholders := make([]string, len(posts))
	arguments := make([]any, len(posts))
	byserial := make(map[string]int, len(posts))
	for index := range posts {
		placeholders[index] = "?"
		arguments[index] = posts[index].Serial
		byserial[posts[index].Serial] = index
	}

	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(tagsForPostsQuery, strings.Join(placeholders, ", ")), arguments...)
	if err != nil {
		return fmt.Errorf("authoring: read tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var serial, name string
		if err := rows.Scan(&serial, &name); err != nil {
			return fmt.Errorf("authoring: scan tag: %w", err)
		}
		if index, ok := byserial[serial]; ok {
			posts[index].Tags = append(posts[index].Tags, name)
		}
	}
	return rows.Err()
}

const postQuery = `
	SELECT p.serial,
	       COALESCE(p.title, ''),
	       COALESCE(p.description, ''),
	       pp.access_policy,
	       pp.password IS NOT NULL AND pp.password <> '',
	       p.created_at,
	       COALESCE(all_time.play_count, 0),
	       COALESCE(this_week.play_count, 0),
	       COALESCE(last_week.play_count, 0)
	  FROM posts AS p
	  JOIN post_policies AS pp ON pp.post_id = p.id
	  LEFT JOIN post_statistics AS all_time
	         ON all_time.post_id = p.id AND all_time.time_range = 'all'
	  LEFT JOIN post_statistics AS this_week
	         ON this_week.post_id = p.id AND this_week.time_range = 'week'
	        AND this_week.start_date = ?
	  LEFT JOIN post_statistics AS last_week
	         ON last_week.post_id = p.id AND last_week.time_range = 'week'
	        AND last_week.start_date = ?
	 WHERE p.user_id = ?
	   AND p.serial = ?
	   AND p.deleted_at IS NULL
	 LIMIT 1`

func (repository *MySQLRepository) Post(ctx context.Context, userID int64, serial string) (Post, error) {
	thisWeek, lastWeek := weekBoundaries(repository.now())

	var (
		post      Post
		createdAt sql.NullTime
	)
	err := repository.database.QueryRowContext(ctx, postQuery,
		thisWeek, lastWeek, userID, serial).Scan(
		&post.Serial, &post.Title, &post.Description, &post.AccessPolicy, &post.HasPassword,
		&createdAt, &post.PlayCount, &post.ThisWeekPlayCount, &post.LastWeekPlayCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Post{}, ErrPostNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("authoring: read post %q: %w", serial, err)
	}
	if createdAt.Valid {
		post.CreatedAt = createdAt.Time
	}

	post.Tags = []string{}
	single := []Post{post}
	if err := repository.attachTags(ctx, single); err != nil {
		return Post{}, err
	}
	return single[0], nil
}

func (repository *MySQLRepository) CreatePost(
	ctx context.Context, userID int64, serial string, draft PostDraft, passwordHash string,
) error {
	tagIDs, err := repository.resolveTagIDs(ctx, CleanTags(draft.Tags))
	if err != nil {
		return err
	}
	return repository.inTransaction(ctx, func(tx *sql.Tx) error {
		now := repository.now()
		result, err := tx.ExecContext(ctx,
			`INSERT INTO posts (user_id, serial, title, description, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			userID, serial, draft.Title, draft.Description, now, now)
		if err != nil {
			return fmt.Errorf("authoring: insert post: %w", err)
		}
		postID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("authoring: new post id: %w", err)
		}

		// NULL rather than "" for a post with no password, because the read path tests
		// `password IS NOT NULL AND password <> ''` and the 5,166 posts that are not
		// password-protected all hold NULL.
		var storedPassword any
		if passwordHash != "" {
			storedPassword = passwordHash
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO post_policies (post_id, access_policy, password, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			postID, draft.AccessPolicy, storedPassword, now, now); err != nil {
			return fmt.Errorf("authoring: insert policy: %w", err)
		}

		if draft.Tags != nil {
			if err := attachTagIDs(ctx, tx, postID, tagIDs, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *MySQLRepository) UpdatePost(
	ctx context.Context, userID int64, serial string, draft PostDraft, passwordHash *string,
) error {
	tagIDs, err := repository.resolveTagIDs(ctx, CleanTags(draft.Tags))
	if err != nil {
		return err
	}
	return repository.inTransaction(ctx, func(tx *sql.Tx) error {
		postID, err := ownedPostID(ctx, tx, userID, serial)
		if err != nil {
			return err
		}
		now := repository.now()

		if _, err := tx.ExecContext(ctx,
			`UPDATE posts SET title = ?, description = ?, updated_at = ? WHERE id = ?`,
			draft.Title, draft.Description, now, postID); err != nil {
			return fmt.Errorf("authoring: update post: %w", err)
		}

		if passwordHash == nil {
			if _, err := tx.ExecContext(ctx,
				`UPDATE post_policies SET access_policy = ?, updated_at = ? WHERE post_id = ?`,
				draft.AccessPolicy, now, postID); err != nil {
				return fmt.Errorf("authoring: update policy: %w", err)
			}
		} else {
			var stored any
			if *passwordHash != "" {
				stored = *passwordHash
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE post_policies SET access_policy = ?, password = ?, updated_at = ?
				  WHERE post_id = ?`,
				draft.AccessPolicy, stored, now, postID); err != nil {
				return fmt.Errorf("authoring: update policy: %w", err)
			}
		}

		if draft.Tags != nil {
			if err := attachTagIDs(ctx, tx, postID, tagIDs, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// resolveTagIDs turns tag names into ids, creating the ones that do not exist.
//
// OUTSIDE THE POST'S TRANSACTION, DELIBERATELY. Inside one, two authors saving the same
// new tag at the same moment deadlock: INSERT IGNORE takes a shared lock on the row it
// collides with, both transactions hold one, and both then want to write the pivot. The
// test that saves one tag onto two posts concurrently reproduced it on the first run —
// Error 1213, every time.
//
// In autocommit each statement is its own transaction and releases its locks at once, so
// no cycle can form. The cost is that a tag created here survives a rolled-back edit,
// which is an unreferenced row in a shared vocabulary table and is exactly what Laravel's
// firstOrCreate did too.
func (repository *MySQLRepository) resolveTagIDs(ctx context.Context, tags []string) ([]int64, error) {
	ids := make([]int64, 0, len(tags))
	for _, name := range tags {
		// INSERT IGNORE turns any error into a warning, a too-long value included: MySQL
		// would truncate a 16-character tag to 15 and store something the author did not
		// type, and the SELECT below would then miss it. Refused here so the reason is
		// named rather than surfacing as a row that is not there.
		if utf8.RuneCountInString(name) > MaxTagNameRunes {
			return nil, &ErrInvalid{Fields: FieldErrors{"tags": []string{CodeTooLong}}}
		}
		now := repository.now()
		if _, err := repository.database.ExecContext(ctx,
			`INSERT IGNORE INTO tags (name, created_at, updated_at) VALUES (?, ?, ?)`,
			name, now, now); err != nil {
			return nil, fmt.Errorf("authoring: create tag %q: %w", name, err)
		}
		var tagID int64
		if err := repository.database.QueryRowContext(ctx,
			`SELECT id FROM tags WHERE name = ? LIMIT 1`, name).Scan(&tagID); err != nil {
			return nil, fmt.Errorf("authoring: read tag %q: %w", name, err)
		}
		ids = append(ids, tagID)
	}
	return ids, nil
}

// attachTagIDs replaces a post's tags with ids resolved before the transaction opened.
func attachTagIDs(ctx context.Context, tx *sql.Tx, postID int64, tagIDs []int64, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id = ?`, postID); err != nil {
		return fmt.Errorf("authoring: detach tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO post_tags (post_id, tag_id, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			postID, tagID, now, now); err != nil {
			return fmt.Errorf("authoring: attach tag %d: %w", tagID, err)
		}
	}
	return nil
}

// DeletePost does what PostService::delete plus its two listeners did, in one
// transaction rather than as three separate effects that could half-apply.
func (repository *MySQLRepository) DeletePost(
	ctx context.Context, userID int64, serial string,
) (int64, error) {
	var postID int64
	err := repository.inTransaction(ctx, func(tx *sql.Tx) error {
		id, err := ownedPostID(ctx, tx, userID, serial)
		if err != nil {
			return err
		}
		postID = id
		now := repository.now()

		if _, err := tx.ExecContext(ctx, `DELETE FROM post_tags WHERE post_id = ?`, id); err != nil {
			return fmt.Errorf("authoring: detach tags: %w", err)
		}
		// The elements go with the post, as DeleteElements did. Soft-deleted, and the
		// pivot rows are left in place — which is what the listener did too, since it
		// called $element->delete() without detaching.
		if _, err := tx.ExecContext(ctx,
			`UPDATE elements AS e
			   JOIN post_elements AS pe ON pe.element_id = e.id
			    SET e.deleted_at = ?, e.updated_at = ?
			  WHERE pe.post_id = ? AND e.deleted_at IS NULL`, now, now, id); err != nil {
			return fmt.Errorf("authoring: delete elements: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM rank_reports WHERE post_id = ?`, id); err != nil {
			return fmt.Errorf("authoring: delete rank reports: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE posts SET deleted_at = ?, updated_at = ? WHERE id = ?`,
			now, now, id); err != nil {
			return fmt.Errorf("authoring: delete post: %w", err)
		}
		return nil
	})
	return postID, err
}

// elementsPageQuery reads a page of a post's elements with each one's rank in THIS post.
//
// The join is on both ids: an element can belong to more than one post, and its rank is
// per post. Reading it with only element_id would show a rank from somewhere else.
const elementsPageQuery = `
	SELECT e.id,
	       COALESCE(e.source_url, ''),
	       COALESCE(e.thumb_url, ''),
	       COALESCE(e.mediumthumb_url, ''),
	       COALESCE(e.lowthumb_url, ''),
	       COALESCE(e.title, ''),
	       e.type,
	       COALESCE(e.video_source, ''),
	       COALESCE(e.video_id, ''),
	       e.video_duration_second,
	       e.video_start_second,
	       e.video_end_second,
	       e.created_at,
	       rr.rank,
	       rr.win_rate,
	       rr.final_win_rate
	  FROM post_elements AS pe
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	  LEFT JOIN rank_reports AS rr ON rr.element_id = e.id AND rr.post_id = pe.post_id
	 WHERE pe.post_id = ?
	   %s
	 ORDER BY %s
	 LIMIT ? OFFSET ?`

const elementsCountQuery = `
	SELECT COUNT(*)
	  FROM post_elements AS pe
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE pe.post_id = ?
	   %s`

func (repository *MySQLRepository) Elements(
	ctx context.Context, userID int64, serial string, query ElementQuery,
) (ElementPage, error) {
	postID, err := ownedPostIDFromDB(ctx, repository.database, userID, serial)
	if err != nil {
		return ElementPage{}, err
	}

	filter := ""
	arguments := []any{postID}
	if query.TitleLike != "" {
		filter = "AND e.title LIKE ?"
		// The wildcards are added here rather than taken from the caller, so a search
		// for "100%" is a search for that text and not a pattern.
		arguments = append(arguments, "%"+escapeLike(query.TitleLike)+"%")
	}

	// The sort is built from a fixed set, never from the caller's string.
	direction := "ASC"
	if query.Descending {
		direction = "DESC"
	}
	order := "e.id " + direction
	if query.SortBy == "title" {
		// id as the tie-break, or two elements with the same title swap places between
		// pages and one of them is never shown.
		order = "e.title " + direction + ", e.id " + direction
	}

	page := ElementPage{Page: query.Page, PerPage: query.PerPage, Elements: []Element{}}
	rows, err := repository.database.QueryContext(ctx,
		fmt.Sprintf(elementsPageQuery, filter, order),
		append(append([]any{}, arguments...), query.PerPage, (query.Page-1)*query.PerPage)...)
	if err != nil {
		return ElementPage{}, fmt.Errorf("authoring: list elements of %q: %w", serial, err)
	}
	defer rows.Close()

	for rows.Next() {
		element, err := scanElement(rows)
		if err != nil {
			return ElementPage{}, err
		}
		page.Elements = append(page.Elements, element)
	}
	if err := rows.Err(); err != nil {
		return ElementPage{}, fmt.Errorf("authoring: list elements of %q: %w", serial, err)
	}

	if err := repository.database.QueryRowContext(ctx,
		fmt.Sprintf(elementsCountQuery, filter), arguments...).Scan(&page.Total); err != nil {
		return ElementPage{}, fmt.Errorf("authoring: count elements of %q: %w", serial, err)
	}
	return page, nil
}

type scanner interface {
	Scan(destination ...any) error
}

func scanElement(row scanner) (Element, error) {
	var (
		element   Element
		duration  sql.NullInt64
		start     sql.NullInt64
		end       sql.NullInt64
		createdAt sql.NullTime
		rank      sql.NullInt64
		winRate   sql.NullFloat64
		finalRate sql.NullFloat64
	)
	if err := row.Scan(&element.ID, &element.SourceURL, &element.ThumbURL, &element.MediumURL,
		&element.LowURL, &element.Title, &element.Type, &element.VideoSource, &element.VideoID,
		&duration, &start, &end, &createdAt, &rank, &winRate, &finalRate); err != nil {
		return Element{}, fmt.Errorf("authoring: scan element: %w", err)
	}
	element.DurationSecs = nullableInt(duration)
	element.StartSecond = nullableInt(start)
	element.EndSecond = nullableInt(end)
	if createdAt.Valid {
		element.CreatedAt = createdAt.Time
	}
	if rank.Valid {
		element.Rank = &ElementRank{
			Rank:         int(rank.Int64),
			WinRate:      winRate.Float64,
			FinalWinRate: finalRate.Float64,
		}
	}
	return element, nil
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}

// elementOwnedQuery is the ownership test for an element: it belongs to at least one
// post the user owns. Mirrors ElementPolicy::update, which asked the same question.
const elementOwnedQuery = `
	SELECT EXISTS (
		SELECT 1
		  FROM post_elements AS pe
		  JOIN posts AS p ON p.id = pe.post_id
		 WHERE pe.element_id = ? AND p.user_id = ? AND p.deleted_at IS NULL)`

func (repository *MySQLRepository) UpdateElement(
	ctx context.Context, userID int64, elementID int64, edit ElementEdit,
) (Element, error) {
	var owned bool
	if err := repository.database.QueryRowContext(ctx, elementOwnedQuery,
		elementID, userID).Scan(&owned); err != nil {
		return Element{}, fmt.Errorf("authoring: check element %d: %w", elementID, err)
	}
	if !owned {
		return Element{}, ErrElementNotFound
	}

	assignments := []string{"updated_at = ?"}
	arguments := []any{repository.now()}
	if edit.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *edit.Title)
	}
	if edit.StartSecond != nil {
		assignments = append(assignments, "video_start_second = ?")
		arguments = append(arguments, *edit.StartSecond)
	}
	if edit.EndSecond != nil {
		assignments = append(assignments, "video_end_second = ?")
		arguments = append(arguments, *edit.EndSecond)
	}
	arguments = append(arguments, elementID)

	if _, err := repository.database.ExecContext(ctx,
		fmt.Sprintf(`UPDATE elements SET %s WHERE id = ? AND deleted_at IS NULL`,
			strings.Join(assignments, ", ")), arguments...); err != nil {
		return Element{}, fmt.Errorf("authoring: update element %d: %w", elementID, err)
	}
	return repository.element(ctx, elementID)
}

// element reads one element back, without a post to rank it in.
const singleElementQuery = `
	SELECT e.id,
	       COALESCE(e.source_url, ''),
	       COALESCE(e.thumb_url, ''),
	       COALESCE(e.mediumthumb_url, ''),
	       COALESCE(e.lowthumb_url, ''),
	       COALESCE(e.title, ''),
	       e.type,
	       COALESCE(e.video_source, ''),
	       COALESCE(e.video_id, ''),
	       e.video_duration_second,
	       e.video_start_second,
	       e.video_end_second,
	       e.created_at,
	       NULL, NULL, NULL
	  FROM elements AS e
	 WHERE e.id = ? AND e.deleted_at IS NULL
	 LIMIT 1`

func (repository *MySQLRepository) element(ctx context.Context, elementID int64) (Element, error) {
	element, err := scanElement(repository.database.QueryRowContext(ctx, singleElementQuery, elementID))
	if errors.Is(err, sql.ErrNoRows) {
		return Element{}, ErrElementNotFound
	}
	return element, err
}

// DeleteElement does what ElementService::delete plus DeleteElementRank did.
func (repository *MySQLRepository) DeleteElement(
	ctx context.Context, userID int64, elementID int64,
) ([]int64, error) {
	var affected []int64
	err := repository.inTransaction(ctx, func(tx *sql.Tx) error {
		var owned bool
		if err := tx.QueryRowContext(ctx, elementOwnedQuery, elementID, userID).Scan(&owned); err != nil {
			return fmt.Errorf("authoring: check element %d: %w", elementID, err)
		}
		if !owned {
			return ErrElementNotFound
		}

		// The posts whose reports this invalidates, read before the rows go.
		rows, err := tx.QueryContext(ctx,
			`SELECT DISTINCT post_id FROM rank_reports WHERE element_id = ?`, elementID)
		if err != nil {
			return fmt.Errorf("authoring: read affected posts: %w", err)
		}
		for rows.Next() {
			var postID int64
			if err := rows.Scan(&postID); err != nil {
				rows.Close()
				return fmt.Errorf("authoring: scan affected post: %w", err)
			}
			affected = append(affected, postID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("authoring: read affected posts: %w", err)
		}

		now := repository.now()
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM post_elements WHERE element_id = ?`, elementID); err != nil {
			return fmt.Errorf("authoring: detach element: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM rank_reports WHERE element_id = ?`, elementID); err != nil {
			return fmt.Errorf("authoring: delete rank reports: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE elements SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`,
			now, now, elementID); err != nil {
			return fmt.Errorf("authoring: delete element: %w", err)
		}
		return nil
	})
	return affected, err
}

// PasswordHash reads the account's bcrypt hash for the deletion confirmation.
func (repository *MySQLRepository) PasswordHash(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := repository.database.QueryRowContext(ctx,
		`SELECT password FROM users WHERE id = ? LIMIT 1`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrPostNotFound
	}
	if err != nil {
		return "", fmt.Errorf("authoring: read password for %d: %w", userID, err)
	}
	return hash, nil
}

const ownedPostIDQuery = `
	SELECT id FROM posts WHERE user_id = ? AND serial = ? AND deleted_at IS NULL LIMIT 1`

func ownedPostID(ctx context.Context, tx *sql.Tx, userID int64, serial string) (int64, error) {
	var postID int64
	err := tx.QueryRowContext(ctx, ownedPostIDQuery, userID, serial).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrPostNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("authoring: resolve post %q: %w", serial, err)
	}
	return postID, nil
}

func ownedPostIDFromDB(ctx context.Context, database *sql.DB, userID int64, serial string) (int64, error) {
	var postID int64
	err := database.QueryRowContext(ctx, ownedPostIDQuery, userID, serial).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrPostNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("authoring: resolve post %q: %w", serial, err)
	}
	return postID, nil
}

// inTransaction runs work in one transaction, at READ COMMITTED.
//
// NOT THE SERVER'S DEFAULT, AND THE REASON IS GAP LOCKS. Under REPEATABLE READ, the
// `DELETE FROM post_tags WHERE post_id = ?` that starts a tag rewrite matches nothing on
// a post that has no tags yet, and an empty-range delete still takes an exclusive gap
// lock before the next record in the primary key. Gap locks do not conflict with each
// other, so several transactions take the same one happily — and then each one's INSERT
// needs an insert-intention lock in that gap, which does conflict. Six authors saving the
// same new tag at once deadlocked every time; InnoDB's own report names the gap lock on
// post_tags PRIMARY as both what each transaction held and what it waited for.
//
// READ COMMITTED does not take those gap locks. Nothing here needs what they protect
// against: every statement is keyed by a post id that no concurrent request shares, and
// none of them reads a range twice. The binary log is in ROW format, which is what makes
// READ COMMITTED legal at all.
func (repository *MySQLRepository) inTransaction(ctx context.Context, work func(*sql.Tx) error) error {
	tx, err := repository.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("authoring: begin: %w", err)
	}
	if err := work(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("authoring: commit: %w", err)
	}
	return nil
}

// weekBoundaries is this week's and last week's start dates, as post_statistics stores
// them.
//
// Monday, because Carbon's startOfWeek is Monday by default and that is what wrote the
// rows. In the application timezone, since the dates are calendar dates.
func weekBoundaries(now time.Time) (thisWeek, lastWeek string) {
	local := now
	weekday := int(local.Weekday())
	if weekday == 0 {
		// Go counts Sunday as 0; Carbon's week starts on Monday, so Sunday is day seven.
		weekday = 7
	}
	monday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).
		AddDate(0, 0, -(weekday - 1))
	return monday.Format("2006-01-02"), monday.AddDate(0, 0, -7).Format("2006-01-02")
}

// escapeLike neutralises the wildcards inside a search term so a literal % or _ is
// matched as itself.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
