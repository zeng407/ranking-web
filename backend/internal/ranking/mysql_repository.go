package ranking

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLRepository implements Repository against the existing schema.
type MySQLRepository struct {
	database *sql.DB
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{database: database}
}

// The four delta queries mirror the Eloquent builders in
// RankService::createElementRank. They aggregate in the database rather than
// pulling rows, because game_1v1_rounds holds roughly 45.9M rows.
//
// `id > ?` is the watermark. COALESCE keeps MAX(id) as 0 rather than NULL when
// nothing matches, so the caller does not need to handle a null scan.
const (
	completedWinQuery = `
		SELECT COUNT(*),
		       COALESCE(MAX(rounds.id), 0),
		       COALESCE(SUM(CASE WHEN rounds.remain_elements = 1 THEN 1 ELSE 0 END), 0)
		  FROM games
		  JOIN game_1v1_rounds rounds ON rounds.game_id = games.id
		 WHERE games.post_id = ?
		   AND games.completed_at IS NOT NULL
		   AND rounds.winner_id = ?
		   AND rounds.id > ?`

	completedLoseQuery = `
		SELECT COUNT(*), COALESCE(MAX(rounds.id), 0)
		  FROM games
		  JOIN game_1v1_rounds rounds ON rounds.game_id = games.id
		 WHERE games.post_id = ?
		   AND games.completed_at IS NOT NULL
		   AND rounds.loser_id = ?
		   AND rounds.id > ?`

	allGamesWinQuery = `
		SELECT COUNT(*), COALESCE(MAX(rounds.id), 0)
		  FROM games
		  JOIN game_1v1_rounds rounds ON rounds.game_id = games.id
		 WHERE games.post_id = ?
		   AND rounds.winner_id = ?
		   AND rounds.id > ?`

	allGamesLoseQuery = `
		SELECT COUNT(*), COALESCE(MAX(rounds.id), 0)
		  FROM games
		  JOIN game_1v1_rounds rounds ON rounds.game_id = games.id
		 WHERE games.post_id = ?
		   AND rounds.loser_id = ?
		   AND rounds.id > ?`
)

func (repository *MySQLRepository) CompletedWinDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error) {
	var delta RoundDelta
	err := repository.database.QueryRowContext(ctx, completedWinQuery, postID, elementID, afterRoundID).
		Scan(&delta.Count, &delta.MaxID, &delta.ChampionCount)
	if err != nil {
		return RoundDelta{}, fmt.Errorf("ranking: completed win delta for post %d element %d: %w", postID, elementID, err)
	}
	return delta, nil
}

func (repository *MySQLRepository) CompletedLoseDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error) {
	return repository.countDelta(ctx, completedLoseQuery, "completed lose", postID, elementID, afterRoundID)
}

func (repository *MySQLRepository) AllGamesWinDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error) {
	return repository.countDelta(ctx, allGamesWinQuery, "all games win", postID, elementID, afterRoundID)
}

func (repository *MySQLRepository) AllGamesLoseDelta(ctx context.Context, postID, elementID, afterRoundID int64) (RoundDelta, error) {
	return repository.countDelta(ctx, allGamesLoseQuery, "all games lose", postID, elementID, afterRoundID)
}

func (repository *MySQLRepository) countDelta(
	ctx context.Context,
	query, label string,
	postID, elementID, afterRoundID int64,
) (RoundDelta, error) {
	var delta RoundDelta
	err := repository.database.QueryRowContext(ctx, query, postID, elementID, afterRoundID).
		Scan(&delta.Count, &delta.MaxID)
	if err != nil {
		return RoundDelta{}, fmt.Errorf("ranking: %s delta for post %d element %d: %w", label, postID, elementID, err)
	}
	return delta, nil
}

// upsertRankQuery relies on ranks_post_element_type_date_unique, added in
// migration 00003.
//
// This replaces Laravel's updateOrCreate, which was a SELECT followed by an
// INSERT or UPDATE with nothing making the pair atomic; two concurrent runs for
// the same element both found no row and both inserted, which had already
// produced duplicate rows. A single statement against the unique key cannot race.
const upsertRankQuery = `
	INSERT INTO ranks
	       (post_id, element_id, rank_type, record_date, win_count, round_count, win_rate, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	    ON DUPLICATE KEY UPDATE
	       win_count = VALUES(win_count),
	       round_count = VALUES(round_count),
	       win_rate = VALUES(win_rate),
	       updated_at = VALUES(updated_at)`

func (repository *MySQLRepository) UpsertRank(ctx context.Context, rank Rank) error {
	if !rank.RankType.Valid() {
		return fmt.Errorf("ranking: unknown rank type %q", rank.RankType)
	}

	// record_date is a DATE column, so only the calendar day is sent; the caller
	// has already resolved it in the application timezone.
	recordDate := rank.RecordDate.Format("2006-01-02")
	now := time.Now()

	_, err := repository.database.ExecContext(ctx, upsertRankQuery,
		rank.PostID, rank.ElementID, string(rank.RankType), recordDate,
		rank.WinCount, rank.RoundCount, rank.WinRate, now, now,
	)
	if err != nil {
		return fmt.Errorf("ranking: upsert %s rank for post %d element %d: %w",
			rank.RankType, rank.PostID, rank.ElementID, err)
	}
	return nil
}
