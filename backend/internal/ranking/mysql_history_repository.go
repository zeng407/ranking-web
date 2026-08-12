package ranking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MySQLHistoryRepository implements HistoryRepository.
type MySQLHistoryRepository struct {
	database *sql.DB
}

func NewMySQLHistoryRepository(database *sql.DB) *MySQLHistoryRepository {
	return &MySQLHistoryRepository{database: database}
}

// Every read filters deleted_at IS NULL. The model uses SoftDeletes, so Eloquent
// applies that scope automatically and omitting it here would make the port see
// refreshed-away rows the original never sees.

const latestHistoryStartDateQuery = `
	SELECT MAX(start_date) FROM rank_report_histories
	 WHERE rank_report_id = ? AND time_range = ? AND deleted_at IS NULL`

func (repository *MySQLHistoryRepository) LatestHistoryStartDate(
	ctx context.Context, rankReportID int64, timeRange HistoryTimeRange,
) (time.Time, error) {
	var latest sql.NullTime
	err := repository.database.QueryRowContext(ctx, latestHistoryStartDateQuery, rankReportID, string(timeRange)).Scan(&latest)
	if err != nil {
		return time.Time{}, fmt.Errorf("ranking: latest history start date: %w", err)
	}
	if !latest.Valid {
		return time.Time{}, nil
	}
	return latest.Time, nil
}

const softDeleteHistoryQuery = `
	UPDATE rank_report_histories SET deleted_at = ?
	 WHERE rank_report_id = ? AND time_range = ? AND deleted_at IS NULL`

func (repository *MySQLHistoryRepository) SoftDeleteHistory(
	ctx context.Context, rankReportID int64, timeRange HistoryTimeRange,
) (int64, error) {
	result, err := repository.database.ExecContext(ctx, softDeleteHistoryQuery,
		time.Now(), rankReportID, string(timeRange))
	if err != nil {
		return 0, fmt.Errorf("ranking: soft delete history: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ranking: count soft deleted history rows: %w", err)
	}
	return affected, nil
}

// firstRankOnOrAfterQuery mirrors getLastRankRecord, which despite its name takes
// the EARLIEST row at or after the date (orderBy ascending, first).
const firstRankOnOrAfterQuery = `
	SELECT record_date, rank_type, win_count, round_count
	  FROM ranks
	 WHERE post_id = ? AND element_id = ? AND record_date >= ? AND rank_type = ?
	 ORDER BY record_date
	 LIMIT 1`

func (repository *MySQLHistoryRepository) FirstRankOnOrAfter(
	ctx context.Context, postID, elementID int64, onOrAfter time.Time, rankType RankType,
) (*DailyRank, error) {
	if !rankType.Valid() {
		return nil, fmt.Errorf("ranking: unknown rank type %q", rankType)
	}

	var (
		rank     DailyRank
		typeName string
	)
	err := repository.database.QueryRowContext(ctx, firstRankOnOrAfterQuery,
		postID, elementID, onOrAfter.Format(dateLayout), string(rankType),
	).Scan(&rank.RecordDate, &typeName, &rank.WinCount, &rank.RoundCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ranking: first rank on or after: %w", err)
	}
	rank.RankType = RankType(typeName)
	return &rank, nil
}

const ranksOnOrAfterQuery = `
	SELECT record_date, rank_type, win_count, round_count
	  FROM ranks
	 WHERE post_id = ? AND element_id = ? AND record_date >= ?
	 ORDER BY record_date`

func (repository *MySQLHistoryRepository) RanksOnOrAfter(
	ctx context.Context, postID, elementID int64, onOrAfter time.Time,
) ([]DailyRank, error) {
	rows, err := repository.database.QueryContext(ctx, ranksOnOrAfterQuery,
		postID, elementID, onOrAfter.Format(dateLayout))
	if err != nil {
		return nil, fmt.Errorf("ranking: query ranks on or after: %w", err)
	}
	defer rows.Close()

	ranks := make([]DailyRank, 0)
	for rows.Next() {
		var (
			rank     DailyRank
			typeName string
		)
		if err := rows.Scan(&rank.RecordDate, &typeName, &rank.WinCount, &rank.RoundCount); err != nil {
			return nil, fmt.Errorf("ranking: scan daily rank: %w", err)
		}
		rank.RankType = RankType(typeName)
		ranks = append(ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate daily ranks: %w", err)
	}
	return ranks, nil
}

const historyDatesPresentQuery = `
	SELECT start_date FROM rank_report_histories
	 WHERE rank_report_id = ? AND time_range = ? AND deleted_at IS NULL`

// HistoryDatesPresent fetches the whole set in one query. The original runs an
// exists() per day inside the timeline loop, which is one round trip per day per
// element.
func (repository *MySQLHistoryRepository) HistoryDatesPresent(
	ctx context.Context, rankReportID int64, timeRange HistoryTimeRange,
) (map[string]struct{}, error) {
	rows, err := repository.database.QueryContext(ctx, historyDatesPresentQuery, rankReportID, string(timeRange))
	if err != nil {
		return nil, fmt.Errorf("ranking: query history dates: %w", err)
	}
	defer rows.Close()

	present := make(map[string]struct{})
	for rows.Next() {
		var startDate time.Time
		if err := rows.Scan(&startDate); err != nil {
			return nil, fmt.Errorf("ranking: scan history date: %w", err)
		}
		present[startDate.Format(dateLayout)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate history dates: %w", err)
	}
	return present, nil
}

// HistoryInsertChunkSize bounds one multi-row insert. A first build walks from the
// post's creation date, which can be hundreds of days.
const HistoryInsertChunkSize = 200

const historyColumns = `(element_id, post_id, rank_report_id, time_range, start_date,
	` + "`rank`" + `, win_count, lose_count, win_rate, champion_count, game_complete_count,
	champion_rate, created_at, updated_at)`

// The table has no unique index on the natural key, so nothing here can rely on
// ON DUPLICATE KEY UPDATE. That absence is deliberate and documented: the model
// soft deletes, so a soft-deleted row and a live row legitimately share the
// natural key, and a plain unique index would break every refresh. Serialisation
// per rank_report_id is what keeps concurrent writers apart, matching Laravel's
// ShouldBeUnique on the same key.

func (repository *MySQLHistoryRepository) InsertHistoryRows(ctx context.Context, rows []HistoryRow) error {
	if len(rows) == 0 {
		return nil
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ranking: begin history insert: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	for start := 0; start < len(rows); start += HistoryInsertChunkSize {
		end := start + HistoryInsertChunkSize
		if end > len(rows) {
			end = len(rows)
		}
		if err := insertHistoryChunk(ctx, transaction, rows[start:end]); err != nil {
			return err
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("ranking: commit history insert: %w", err)
	}
	return nil
}

func insertHistoryChunk(ctx context.Context, transaction *sql.Tx, rows []HistoryRow) error {
	placeholders := make([]string, 0, len(rows))
	arguments := make([]any, 0, len(rows)*14)
	now := time.Now()
	for _, row := range rows {
		if !row.TimeRange.Valid() {
			return fmt.Errorf("ranking: unknown history time range %q", row.TimeRange)
		}
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		arguments = append(arguments,
			row.ElementID, row.PostID, row.RankReportID, string(row.TimeRange),
			row.StartDate.Format(dateLayout), row.Rank, row.WinCount, row.LoseCount,
			row.WinRate, row.ChampionCount, row.GameCompleteCount, row.ChampionRate,
			now, now,
		)
	}

	query := "INSERT INTO rank_report_histories " + historyColumns +
		" VALUES " + strings.Join(placeholders, ", ")
	if _, err := transaction.ExecContext(ctx, query, arguments...); err != nil {
		return fmt.Errorf("ranking: insert %d history rows: %w", len(rows), err)
	}
	return nil
}

// UpsertHistoryRow writes one row, updating the live row for the natural key when
// one exists.
//
// Implemented as UPDATE-then-INSERT rather than ON DUPLICATE KEY UPDATE because
// there is no unique index to conflict on. Concurrency is handled a level up by
// the per-rank_report serialisation lock, so the gap between the two statements is
// not reachable by a second writer for the same key.
func (repository *MySQLHistoryRepository) UpsertHistoryRow(ctx context.Context, row HistoryRow) error {
	if !row.TimeRange.Valid() {
		return fmt.Errorf("ranking: unknown history time range %q", row.TimeRange)
	}

	const updateQuery = `
		UPDATE rank_report_histories
		   SET win_count = ?, lose_count = ?, win_rate = ?, champion_count = ?,
		       game_complete_count = ?, champion_rate = ?, updated_at = ?
		 WHERE element_id = ? AND post_id = ? AND rank_report_id = ?
		   AND time_range = ? AND start_date = ? AND deleted_at IS NULL`

	now := time.Now()
	startDate := row.StartDate.Format(dateLayout)

	result, err := repository.database.ExecContext(ctx, updateQuery,
		row.WinCount, row.LoseCount, row.WinRate, row.ChampionCount,
		row.GameCompleteCount, row.ChampionRate, now,
		row.ElementID, row.PostID, row.RankReportID, string(row.TimeRange), startDate,
	)
	if err != nil {
		return fmt.Errorf("ranking: update history row: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ranking: count updated history rows: %w", err)
	}
	if affected > 0 {
		return nil
	}

	query := "INSERT INTO rank_report_histories " + historyColumns +
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err = repository.database.ExecContext(ctx, query,
		row.ElementID, row.PostID, row.RankReportID, string(row.TimeRange), startDate,
		row.Rank, row.WinCount, row.LoseCount, row.WinRate,
		row.ChampionCount, row.GameCompleteCount, row.ChampionRate, now, now,
	)
	if err != nil {
		return fmt.Errorf("ranking: insert history row: %w", err)
	}
	return nil
}

// recentVotesQuery returns the element's most recent rounds for the post, newest
// first.
//
// The OR over winner_id and loser_id is what the original does. It cannot use
// idx_rounds_game_winner and idx_rounds_game_loser at once, so MySQL takes an
// index merge; the LIMIT keeps it bounded.
const recentVotesQuery = `
	SELECT rounds.id, rounds.winner_id = ? AS won
	  FROM game_1v1_rounds rounds
	  JOIN games ON games.id = rounds.game_id
	 WHERE games.post_id = ?
	   AND (rounds.winner_id = ? OR rounds.loser_id = ?)
	 ORDER BY rounds.id DESC
	 LIMIT ?`

func (repository *MySQLHistoryRepository) RecentVotes(
	ctx context.Context, postID, elementID int64, limit int,
) ([]VoteOutcome, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("ranking: recent votes limit must be positive, got %d", limit)
	}

	rows, err := repository.database.QueryContext(ctx, recentVotesQuery,
		elementID, postID, elementID, elementID, limit)
	if err != nil {
		return nil, fmt.Errorf("ranking: query recent votes: %w", err)
	}
	defer rows.Close()

	outcomes := make([]VoteOutcome, 0, limit)
	for rows.Next() {
		var outcome VoteOutcome
		if err := rows.Scan(&outcome.RoundID, &outcome.Won); err != nil {
			return nil, fmt.Errorf("ranking: scan recent vote: %w", err)
		}
		outcomes = append(outcomes, outcome)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking: iterate recent votes: %w", err)
	}
	return outcomes, nil
}
