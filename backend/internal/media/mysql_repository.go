package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MySQLElementRepository implements ElementRepository.
type MySQLElementRepository struct {
	database *sql.DB
}

func NewMySQLElementRepository(database *sql.DB) *MySQLElementRepository {
	return &MySQLElementRepository{database: database}
}

// thumbnailColumns are the only columns these jobs may write. The column name is
// interpolated into the UPDATE, so it must come from this set and never from a
// message payload.
var thumbnailColumns = map[string]struct{}{
	"thumb_url":       {},
	"lowthumb_url":    {},
	"mediumthumb_url": {},
}

const elementColumns = `id, type, video_source, source_url, thumb_url, lowthumb_url, mediumthumb_url, path`

// findElementQuery does not filter deleted_at: an element soft-deleted between
// dispatch and execution should be skipped by the caller, and reading it is how the
// caller learns that. The write paths check separately.
const findElementQuery = `SELECT ` + elementColumns + ` FROM elements WHERE id = ? AND deleted_at IS NULL`

func (repository *MySQLElementRepository) FindElement(ctx context.Context, elementID int64) (*Element, error) {
	row := repository.database.QueryRowContext(ctx, findElementQuery, elementID)

	element, err := scanElement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("media: find element %d: %w", elementID, err)
	}
	return element, nil
}

type rowScanner interface {
	Scan(destination ...any) error
}

func scanElement(scanner rowScanner) (*Element, error) {
	var (
		element        Element
		videoSource    sql.NullString
		sourceURL      sql.NullString
		thumbURL       sql.NullString
		lowThumbURL    sql.NullString
		mediumThumbURL sql.NullString
		path           sql.NullString
	)
	err := scanner.Scan(&element.ID, &element.Type, &videoSource, &sourceURL,
		&thumbURL, &lowThumbURL, &mediumThumbURL, &path)
	if err != nil {
		return nil, err
	}

	for target, value := range map[**string]sql.NullString{
		&element.VideoSource:    videoSource,
		&element.SourceURL:      sourceURL,
		&element.ThumbURL:       thumbURL,
		&element.LowThumbURL:    lowThumbURL,
		&element.MediumThumbURL: mediumThumbURL,
		&element.Path:           path,
	} {
		if value.Valid {
			stored := value.String
			*target = &stored
		}
	}
	return &element, nil
}

func (repository *MySQLElementRepository) SetThumbnailURL(
	ctx context.Context, elementID int64, column, url string,
) error {
	if _, allowed := thumbnailColumns[column]; !allowed {
		// The column is interpolated below, so an unexpected value must never reach
		// the query.
		return fmt.Errorf("media: %q is not a writable thumbnail column", column)
	}
	if url == "" {
		return fmt.Errorf("media: refusing to write an empty url to %s of element %d", column, elementID)
	}

	query := fmt.Sprintf("UPDATE elements SET %s = ?, updated_at = ? WHERE id = ?", column)
	if _, err := repository.database.ExecContext(ctx, query, url, time.Now(), elementID); err != nil {
		return fmt.Errorf("media: set %s for element %d: %w", column, elementID, err)
	}
	return nil
}

// elementsMissingThumbnailQuery mirrors ThumbnailExecutor: image elements whose
// column is null, newest first.
//
// The soft-delete filter is added explicitly; Eloquent applies it through the
// model's scope.
func (repository *MySQLElementRepository) ElementsMissingThumbnail(
	ctx context.Context, column string, limit int,
) ([]Element, error) {
	if _, allowed := thumbnailColumns[column]; !allowed {
		return nil, fmt.Errorf("media: %q is not a thumbnail column", column)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("media: limit must be positive, got %d", limit)
	}

	query := fmt.Sprintf(
		`SELECT %s FROM elements
		  WHERE type = 'image' AND %s IS NULL AND deleted_at IS NULL
		  ORDER BY id DESC LIMIT ?`, elementColumns, column)

	rows, err := repository.database.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("media: query elements missing %s: %w", column, err)
	}
	defer rows.Close()

	return scanElements(rows)
}

// deletedElementsWithFilesQuery mirrors removeDeletedFiles: soft-deleted elements
// that still have a stored path. Clearing path is what removes them from this set.
const deletedElementsWithFilesQuery = `
	SELECT ` + elementColumns + ` FROM elements
	 WHERE deleted_at IS NOT NULL AND path IS NOT NULL
	 LIMIT ?`

func (repository *MySQLElementRepository) DeletedElementsWithFiles(
	ctx context.Context, limit int,
) ([]Element, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("media: limit must be positive, got %d", limit)
	}

	rows, err := repository.database.QueryContext(ctx, deletedElementsWithFilesQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("media: query deleted elements: %w", err)
	}
	defer rows.Close()

	return scanElements(rows)
}

func (repository *MySQLElementRepository) ClearElementPath(ctx context.Context, elementID int64) error {
	_, err := repository.database.ExecContext(ctx,
		`UPDATE elements SET path = NULL, updated_at = ? WHERE id = ?`, time.Now(), elementID)
	if err != nil {
		return fmt.Errorf("media: clear path for element %d: %w", elementID, err)
	}
	return nil
}

func scanElements(rows *sql.Rows) ([]Element, error) {
	elements := make([]Element, 0)
	for rows.Next() {
		element, err := scanElement(rows)
		if err != nil {
			return nil, fmt.Errorf("media: scan element: %w", err)
		}
		elements = append(elements, *element)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("media: iterate elements: %w", err)
	}
	return elements, nil
}
