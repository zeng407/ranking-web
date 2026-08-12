package ranking

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MySQLHistoryRankRepository implements HistoryRankRepository.
type MySQLHistoryRankRepository struct {
	database *sql.DB
}

func NewMySQLHistoryRankRepository(database *sql.DB) *MySQLHistoryRankRepository {
	return &MySQLHistoryRankRepository{database: database}
}

// historyRowsForRankingQuery selects the live rows for one post, range and date.
//
// The ordering is done in Go rather than SQL so the deterministic tie-break on
// element_id is applied by the same code the unit tests exercise.
const historyRowsForRankingQuery = "SELECT id, element_id, win_rate, champion_rate, win_count, `rank`" +
	` FROM rank_report_histories
	 WHERE post_id = ? AND time_range = ? AND start_date = ? AND deleted_at IS NULL`

func (repository *MySQLHistoryRankRepository) HistoryRowsForRanking(
	ctx context.Context, postID int64, timeRange HistoryTimeRange, startDate time.Time,
) ([]RankedHistoryRow, error) {
	if !timeRange.Valid() {
		return nil, fmt.Errorf("ranking: unknown history time range %q", timeRange)
	}

	rows, err := repository.database.QueryContext(ctx, historyRowsForRankingQuery,
		postID, string(timeRange), startDate.Format(dateLayout))
	if err != nil {
		return nil, fmt.Errorf("ranking: query history rows for ranking: %w", err)
	}
	defer rows.Close()

	ranked := make([]RankedHistoryRow, 0)
	for rows.Next() {
		var row RankedHistoryRow
		if err := rows.Scan(&row.ID, &row.ElementID, &row.WinRate, &row.ChampionRate, &row.WinCount, &row.Rank); err != nil {
			return nil, fmt.Errorf("ranking: scan history row: %w", err)
		}
		ranked = append(ranked, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate history rows: %w", err)
	}
	return ranked, nil
}

// ApplyHistoryRanks writes the ranks in batched statements.
//
// The original issues one UPDATE per row inside a Collection::each, which is one
// round trip per element; a post with 866 elements means 866 statements for a
// single date. This uses a CASE over the primary key so a chunk is one statement.
func (repository *MySQLHistoryRankRepository) ApplyHistoryRanks(ctx context.Context, rows []RankedHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ranking: begin history rank update: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	for start := 0; start < len(rows); start += HistoryRankChunkSize {
		end := start + HistoryRankChunkSize
		if end > len(rows) {
			end = len(rows)
		}
		if err := applyHistoryRankChunk(ctx, transaction, rows[start:end]); err != nil {
			return err
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("ranking: commit history rank update: %w", err)
	}
	return nil
}

func applyHistoryRankChunk(ctx context.Context, transaction *sql.Tx, rows []RankedHistoryRow) error {
	cases := make([]string, 0, len(rows))
	ids := make([]string, 0, len(rows))
	arguments := make([]any, 0, len(rows)*2+1+len(rows))

	for _, row := range rows {
		cases = append(cases, "WHEN ? THEN ?")
		arguments = append(arguments, row.ID, row.Rank)
	}
	arguments = append(arguments, time.Now())
	for _, row := range rows {
		ids = append(ids, "?")
		arguments = append(arguments, row.ID)
	}

	// Ordered by id in the IN list is not enough to guarantee lock order, so the
	// caller is expected to have sorted; see ApplyHistoryRanks' doc.
	query := "UPDATE rank_report_histories SET `rank` = CASE id " +
		strings.Join(cases, " ") +
		" END, updated_at = ? WHERE id IN (" + strings.Join(ids, ", ") + ")"

	if _, err := transaction.ExecContext(ctx, query, arguments...); err != nil {
		return fmt.Errorf("ranking: update %d history ranks: %w", len(rows), err)
	}
	return nil
}

// purgeSelectQuery finds the ids to remove.
//
// It is deliberately a select-then-delete-by-id rather than a single
// DELETE ... LIMIT, matching the original, so the bounded set is known before
// anything is removed.
//
// The select filters deleted_at IS NULL because Eloquent's scope does. That means
// rows already soft-deleted are never selected for purging and stay forever; the
// restored dump holds 550 such rows, all past retention. Fixing that is a separate
// decision, not a silent behaviour change here.
const purgeSelectQuery = `
	SELECT id FROM rank_report_histories
	 WHERE post_id = ? AND start_date < ? AND deleted_at IS NULL
	 LIMIT ?`

func (repository *MySQLHistoryRankRepository) PurgeHistoryOlderThan(
	ctx context.Context, postID int64, cutoff time.Time, limit int,
) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("ranking: purge limit must be positive, got %d", limit)
	}

	rows, err := repository.database.QueryContext(ctx, purgeSelectQuery,
		postID, cutoff.Format(dateLayout), limit)
	if err != nil {
		return 0, fmt.Errorf("ranking: select history to purge: %w", err)
	}

	ids := make([]any, 0, limit)
	placeholders := make([]string, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ranking: scan history id: %w", err)
		}
		ids = append(ids, id)
		placeholders = append(placeholders, "?")
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("ranking: iterate history ids: %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		return 0, nil
	}

	// A hard delete, as forceDelete() does. History past retention is not kept as
	// soft-deleted rows; that would defeat the purpose of the purge.
	query := "DELETE FROM rank_report_histories WHERE id IN (" + strings.Join(placeholders, ", ") + ")"
	result, err := repository.database.ExecContext(ctx, query, ids...)
	if err != nil {
		return 0, fmt.Errorf("ranking: delete %d history rows: %w", len(ids), err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ranking: count purged history rows: %w", err)
	}
	return removed, nil
}
