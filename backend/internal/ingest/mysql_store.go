package ingest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MySQLStore implements Store.
type MySQLStore struct {
	database *sql.DB
	now      func() time.Time
}

func NewMySQLStore(database *sql.DB) *MySQLStore {
	return &MySQLStore{database: database, now: time.Now}
}

// postForOwnerQuery resolves the serial and counts what is already attached, in one round
// trip. Ownership is in the WHERE clause, as everywhere else in the editor: a serial that
// is not the caller's does not resolve.
const postForOwnerQuery = `
	SELECT p.id,
	       (SELECT COUNT(*)
	          FROM post_elements AS pe
	          JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	         WHERE pe.post_id = p.id)
	  FROM posts AS p
	 WHERE p.user_id = ? AND p.serial = ? AND p.deleted_at IS NULL
	 LIMIT 1`

func (store *MySQLStore) PostForOwner(
	ctx context.Context, userID int64, serial string,
) (int64, int, error) {
	var (
		postID   int64
		elements int
	)
	err := store.database.QueryRowContext(ctx, postForOwnerQuery, userID, serial).
		Scan(&postID, &elements)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrPostNotFound
	}
	if err != nil {
		return 0, 0, fmt.Errorf("ingest: resolve post %q: %w", serial, err)
	}
	return postID, elements, nil
}

// CreateElement writes the element and the pivot together.
//
// One transaction, because an element with no pivot row belongs to no post: nothing reads
// it, nothing lists it, and the deletion paths — which all start from a post — would never
// find it either. The original wrote them through a relation that did the same thing.
func (store *MySQLStore) CreateElement(ctx context.Context, element NewElement) (Stored, error) {
	tx, err := store.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Stored{}, fmt.Errorf("ingest: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := store.now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO elements (path, source_url, thumb_url, title, type,
		                       video_source, video_id, video_duration_second,
		                       video_start_second, video_end_second, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullable(element.Path), element.SourceURL, element.ThumbURL, element.Title, element.Type,
		nullable(element.VideoSource), nullable(element.VideoID),
		element.DurationSecs, element.StartSecond, element.EndSecond, now, now)
	if err != nil {
		return Stored{}, fmt.Errorf("ingest: insert element: %w", err)
	}
	elementID, err := result.LastInsertId()
	if err != nil {
		return Stored{}, fmt.Errorf("ingest: new element id: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO post_elements (post_id, element_id) VALUES (?, ?)`,
		element.PostID, elementID); err != nil {
		return Stored{}, fmt.Errorf("ingest: attach element: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Stored{}, fmt.Errorf("ingest: commit: %w", err)
	}

	return Stored{
		ID: elementID, SourceURL: element.SourceURL, ThumbURL: element.ThumbURL,
		Title: element.Title, Type: element.Type,
	}, nil
}

// elementForOwnerQuery is the ownership predicate of internal/authoring, plus the serial
// the object key needs. An element can sit on more than one of the owner's posts; the
// lowest post id wins so the same element always keys under the same directory.
const elementForOwnerQuery = `
	SELECT p.serial, e.title
	  FROM post_elements AS pe
	  JOIN posts AS p ON p.id = pe.post_id AND p.deleted_at IS NULL
	  JOIN elements AS e ON e.id = pe.element_id AND e.deleted_at IS NULL
	 WHERE pe.element_id = ? AND p.user_id = ?
	 ORDER BY p.id
	 LIMIT 1`

func (store *MySQLStore) ElementForOwner(
	ctx context.Context, userID, elementID int64,
) (string, string, error) {
	var serial, title string
	err := store.database.QueryRowContext(ctx, elementForOwnerQuery, elementID, userID).
		Scan(&serial, &title)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrElementNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("ingest: resolve element %d: %w", elementID, err)
	}
	return serial, title, nil
}

// ReplaceElementMedia points one element at a new file.
//
// The columns cleared here are the ones Laravel's `path_id` branch cleared: everything that
// described the old medium. Leaving video_id or video_start_second behind on an element
// that is now an image would have the players read a video that is no longer there.
func (store *MySQLStore) ReplaceElementMedia(
	ctx context.Context, elementID int64, media ReplacementMedia,
) error {
	result, err := store.database.ExecContext(ctx,
		`UPDATE elements
		    SET path = ?, source_url = ?, thumb_url = ?, type = ?,
		        mediumthumb_url = NULL, lowthumb_url = NULL,
		        video_source = ?, video_id = NULL, video_duration_second = NULL,
		        video_start_second = NULL, video_end_second = NULL,
		        updated_at = ?
		  WHERE id = ? AND deleted_at IS NULL`,
		nullable(media.Path), media.SourceURL, media.ThumbURL, media.Type,
		nullable(media.VideoSource), store.now(), elementID)
	if err != nil {
		return fmt.Errorf("ingest: replace element %d media: %w", elementID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ingest: replace element %d media: %w", elementID, err)
	}
	if affected == 0 {
		// Deleted between the ownership read and this statement. MySQL reports no
		// affected row for an update that changes nothing too, which cannot happen here:
		// the key carries a fresh uuid, so path and source_url always differ.
		return ErrElementNotFound
	}
	return nil
}

// nullable writes NULL for an empty string, which is what the columns hold for "there is
// none" — video_source on an image, path on a remote video.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
