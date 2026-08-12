package ranking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQLReportRepository implements ReportRepository.
type MySQLReportRepository struct {
	database *sql.DB
}

func NewMySQLReportRepository(database *sql.DB) *MySQLReportRepository {
	return &MySQLReportRepository{database: database}
}

// baseRanksQuery mirrors the Eloquent builder: the post's ranks for one record
// date, excluding soft-deleted elements. The join replaces whereHas('element').
const baseRanksQuery = `
	SELECT r.element_id, r.rank_type, r.win_rate, r.win_count
	  FROM ranks r
	  JOIN elements e ON e.id = r.element_id AND e.deleted_at IS NULL
	 WHERE r.post_id = ? AND r.record_date = ?`

func (repository *MySQLReportRepository) BaseRanks(ctx context.Context, postID int64, recordDate time.Time) ([]BaseRank, error) {
	rows, err := repository.database.QueryContext(ctx, baseRanksQuery, postID, recordDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("ranking: query base ranks: %w", err)
	}
	defer rows.Close()

	ranks := make([]BaseRank, 0)
	for rows.Next() {
		var rank BaseRank
		var rankType string
		if err := rows.Scan(&rank.ElementID, &rankType, &rank.WinRate, &rank.WinCount); err != nil {
			return nil, fmt.Errorf("ranking: scan base rank: %w", err)
		}
		rank.RankType = RankType(rankType)
		if !rank.RankType.Valid() {
			// An unknown enum value would silently drop out of both position maps,
			// producing reports that look complete but are not.
			return nil, fmt.Errorf("ranking: unknown rank type %q for element %d", rankType, rank.ElementID)
		}
		ranks = append(ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate base ranks: %w", err)
	}
	return ranks, nil
}

const existingReportsQuery = `
	SELECT rr.element_id, rr.final_win_position, rr.final_win_rate,
	       rr.win_position, rr.win_rate, rr.created_at
	  FROM rank_reports rr
	  JOIN elements e ON e.id = rr.element_id AND e.deleted_at IS NULL
	 WHERE rr.post_id = ?`

func (repository *MySQLReportRepository) ExistingReports(ctx context.Context, postID int64) (map[int64]ExistingReport, error) {
	rows, err := repository.database.QueryContext(ctx, existingReportsQuery, postID)
	if err != nil {
		return nil, fmt.Errorf("ranking: query existing reports: %w", err)
	}
	defer rows.Close()

	reports := make(map[int64]ExistingReport)
	for rows.Next() {
		var (
			report           ExistingReport
			finalWinPosition sql.NullInt64
			finalWinRate     sql.NullFloat64
			winPosition      sql.NullInt64
			winRate          sql.NullFloat64
			createdAt        sql.NullTime
		)
		err := rows.Scan(&report.ElementID, &finalWinPosition, &finalWinRate, &winPosition, &winRate, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("ranking: scan existing report: %w", err)
		}
		if finalWinPosition.Valid {
			report.FinalWinPosition = &finalWinPosition.Int64
		}
		if finalWinRate.Valid {
			report.FinalWinRate = &finalWinRate.Float64
		}
		if winPosition.Valid {
			report.WinPosition = &winPosition.Int64
		}
		if winRate.Valid {
			report.WinRate = &winRate.Float64
		}
		if createdAt.Valid {
			report.CreatedAt = &createdAt.Time
		}
		reports[report.ElementID] = report
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate existing reports: %w", err)
	}
	return reports, nil
}

// hideDeletedElementReportsQuery marks reports whose element has been soft
// deleted so they stop appearing, without deleting the history.
//
// Note it is one-way, matching the original: nothing ever sets hidden back to
// false, because an element is never un-deleted.
const hideDeletedElementReportsQuery = `
	UPDATE rank_reports rr
	  JOIN elements e ON e.id = rr.element_id
	   SET rr.hidden = 1
	 WHERE rr.post_id = ? AND e.deleted_at IS NOT NULL AND rr.hidden = 0`

// UpsertReports writes the rows and hides reports for deleted elements in one
// transaction, retrying on deadlock.
//
// Rows must already be ordered by element_id; SortReportRowsForWrite does that,
// and consistent lock ordering is what keeps concurrent runs from deadlocking.
func (repository *MySQLReportRepository) UpsertReports(ctx context.Context, postID int64, rows []ReportRow) (int64, error) {
	var hidden int64

	for attempt := 1; attempt <= ReportTransactionAttempts; attempt++ {
		var err error
		hidden, err = repository.upsertOnce(ctx, postID, rows)
		if err == nil {
			return hidden, nil
		}
		if !isRetryableLockError(err) || attempt == ReportTransactionAttempts {
			return 0, err
		}
		// No backoff: a deadlock is resolved the moment the loser rolls back, and
		// the original retries immediately too.
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}
	return hidden, nil
}

func (repository *MySQLReportRepository) upsertOnce(ctx context.Context, postID int64, rows []ReportRow) (int64, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ranking: begin report transaction: %w", err)
	}
	defer func() {
		// Safe after a successful commit: Rollback then returns sql.ErrTxDone,
		// which is ignored.
		_ = transaction.Rollback()
	}()

	for start := 0; start < len(rows); start += ReportUpsertChunkSize {
		end := start + ReportUpsertChunkSize
		if end > len(rows) {
			end = len(rows)
		}
		if err := upsertReportChunk(ctx, transaction, rows[start:end]); err != nil {
			return 0, err
		}
	}

	result, err := transaction.ExecContext(ctx, hideDeletedElementReportsQuery, postID)
	if err != nil {
		return 0, fmt.Errorf("ranking: hide reports for deleted elements: %w", err)
	}
	hidden, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ranking: count hidden reports: %w", err)
	}

	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("ranking: commit report transaction: %w", err)
	}
	return hidden, nil
}

// upsertReportChunk writes one chunk against unique_post_element.
//
// `hidden` and `created_at` are deliberately absent from the update list, exactly
// as in the original: re-running the report must not un-hide a deleted element's
// row, and must not rewrite when the report first appeared.
func upsertReportChunk(ctx context.Context, transaction *sql.Tx, rows []ReportRow) error {
	if len(rows) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(rows))
	arguments := make([]any, 0, len(rows)*9)
	for _, row := range rows {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		arguments = append(arguments,
			row.PostID, row.ElementID, row.Rank,
			nullableInt64(row.FinalWinPosition), row.FinalWinRate,
			nullableInt64(row.WinPosition), row.WinRate,
			row.CreatedAt, row.UpdatedAt,
		)
	}

	query := `
		INSERT INTO rank_reports
		       (post_id, element_id, ` + "`rank`" + `, final_win_position, final_win_rate,
		        win_position, win_rate, created_at, updated_at)
		VALUES ` + strings.Join(placeholders, ", ") + `
		    ON DUPLICATE KEY UPDATE
		       ` + "`rank`" + ` = VALUES(` + "`rank`" + `),
		       final_win_position = VALUES(final_win_position),
		       final_win_rate = VALUES(final_win_rate),
		       win_position = VALUES(win_position),
		       win_rate = VALUES(win_rate),
		       updated_at = VALUES(updated_at)`

	if _, err := transaction.ExecContext(ctx, query, arguments...); err != nil {
		return fmt.Errorf("ranking: upsert %d report rows: %w", len(rows), err)
	}
	return nil
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// MySQL error numbers worth retrying: 1213 is a deadlock, 1205 a lock wait
// timeout. Both mean "try again", not "the write is wrong".
const (
	mysqlErrDeadlock        = 1213
	mysqlErrLockWaitTimeout = 1205
)

func isRetryableLockError(err error) bool {
	var mysqlError *mysql.MySQLError
	if !errors.As(err, &mysqlError) {
		return false
	}
	return mysqlError.Number == mysqlErrDeadlock || mysqlError.Number == mysqlErrLockWaitTimeout
}
