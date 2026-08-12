package ranking

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// Service computes element ranks and rank reports.
type Service struct {
	repository   Repository
	reports      ReportRepository
	history      HistoryRepository
	historyRanks HistoryRankRepository
	posts        PostRepository
	freshness    FreshnessStore
	pending      PendingDatesStore
	stats        StatsStore
	logger       *slog.Logger
	// now supplies the record date. It must be evaluated in the application
	// timezone: `ranks.record_date` is a DATE, and Laravel's today() uses
	// Asia/Taipei, so a UTC clock would file rows under the wrong day for eight
	// hours out of every twenty-four.
	now func() time.Time
}

type Options struct {
	Repository Repository
	// Reports is required only for CreateRankReports.
	Reports ReportRepository
	// History and Pending are required only for the history builders.
	History HistoryRepository
	Pending PendingDatesStore
	// HistoryRanks is required only for rank assignment and purging.
	HistoryRanks HistoryRankRepository
	// Posts and Freshness are required only for the daily sweeps.
	Posts     PostRepository
	Freshness FreshnessStore
	Stats     StatsStore
	Logger    *slog.Logger
	// Location must be explicit for the reason described on Service.now.
	Location *time.Location
	// Now is injectable for tests; it defaults to time.Now.
	Now func() time.Time
}

func NewService(options Options) (*Service, error) {
	if options.Repository == nil {
		return nil, fmt.Errorf("ranking: repository is required")
	}
	if options.Stats == nil {
		return nil, fmt.Errorf("ranking: stats store is required")
	}
	if options.Location == nil {
		return nil, fmt.Errorf("ranking: an explicit timezone is required")
	}
	clock := options.Now
	if clock == nil {
		clock = time.Now
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	location := options.Location

	return &Service{
		repository:   options.Repository,
		reports:      options.Reports,
		history:      options.History,
		historyRanks: options.HistoryRanks,
		posts:        options.Posts,
		freshness:    options.Freshness,
		pending:      options.Pending,
		stats:        options.Stats,
		logger:       logger,
		now:          func() time.Time { return clock().In(location) },
	}, nil
}

// UpdateElementRank recomputes the champion and pk_king rows for one element.
//
// Port of RankService::createElementRank. The order of operations matters: the
// memo is written only after both deltas have been read, and the rank rows carry
// absolute totals so a partial failure converges on the next run.
func (service *Service) UpdateElementRank(ctx context.Context, postID, elementID int64) error {
	if postID <= 0 || elementID <= 0 {
		return fmt.Errorf("ranking: post id and element id are required, got post=%d element=%d", postID, elementID)
	}

	stats, err := service.stats.Get(ctx, postID, elementID)
	if err != nil {
		return fmt.Errorf("ranking: read stats for post %d element %d: %w", postID, elementID, err)
	}

	// --- Champion: rounds of completed games only ---
	championWins, err := service.repository.CompletedWinDelta(ctx, postID, elementID, stats.ChampionMaxWinID)
	if err != nil {
		return fmt.Errorf("ranking: completed win delta: %w", err)
	}
	if championWins.Count > 0 {
		stats.ChampionRoundWins += championWins.Count
		stats.ChampionGameWins += championWins.ChampionCount
		stats.ChampionMaxWinID = maxInt64(stats.ChampionMaxWinID, championWins.MaxID)
	}

	championLoses, err := service.repository.CompletedLoseDelta(ctx, postID, elementID, stats.ChampionMaxLoseID)
	if err != nil {
		return fmt.Errorf("ranking: completed lose delta: %w", err)
	}
	if championLoses.Count > 0 {
		stats.ChampionRoundLoses += championLoses.Count
		stats.ChampionMaxLoseID = maxInt64(stats.ChampionMaxLoseID, championLoses.MaxID)
	}

	// --- PK king: rounds of all games, complete or not ---
	pkWins, err := service.repository.AllGamesWinDelta(ctx, postID, elementID, stats.PKMaxWinID)
	if err != nil {
		return fmt.Errorf("ranking: all games win delta: %w", err)
	}
	if pkWins.Count > 0 {
		stats.PKWinCount += pkWins.Count
		stats.PKMaxWinID = maxInt64(stats.PKMaxWinID, pkWins.MaxID)
	}

	pkLoses, err := service.repository.AllGamesLoseDelta(ctx, postID, elementID, stats.PKMaxLoseID)
	if err != nil {
		return fmt.Errorf("ranking: all games lose delta: %w", err)
	}
	if pkLoses.Count > 0 {
		stats.PKLoseCount += pkLoses.Count
		stats.PKMaxLoseID = maxInt64(stats.PKMaxLoseID, pkLoses.MaxID)
	}

	recordDate := service.now()

	// Champion rows are written only when the element both played completed
	// rounds and won at least one game outright, matching the Laravel guard. An
	// element that never won has no champion row rather than a zero one.
	championRounds := stats.ChampionRoundWins + stats.ChampionRoundLoses
	if championRounds > 0 && stats.ChampionGameWins > 0 {
		err := service.repository.UpsertRank(ctx, Rank{
			PostID:     postID,
			ElementID:  elementID,
			RankType:   RankTypeChampion,
			RecordDate: recordDate,
			WinCount:   stats.ChampionGameWins,
			RoundCount: championRounds,
			WinRate:    WinRate(stats.ChampionGameWins, championRounds),
		})
		if err != nil {
			return fmt.Errorf("ranking: upsert champion rank: %w", err)
		}
	}

	pkRounds := stats.PKWinCount + stats.PKLoseCount
	if pkRounds > 0 {
		err := service.repository.UpsertRank(ctx, Rank{
			PostID:     postID,
			ElementID:  elementID,
			RankType:   RankTypePKKing,
			RecordDate: recordDate,
			WinCount:   stats.PKWinCount,
			RoundCount: pkRounds,
			WinRate:    WinRate(stats.PKWinCount, pkRounds),
		})
		if err != nil {
			return fmt.Errorf("ranking: upsert pk_king rank: %w", err)
		}
	}

	// Written last. If this fails the rank rows still hold correct absolute
	// totals, and the next run recounts from the old watermark to the same
	// values.
	if err := service.stats.Put(ctx, postID, elementID, stats); err != nil {
		return fmt.Errorf("ranking: write stats for post %d element %d: %w", postID, elementID, err)
	}
	return nil
}

// WinRate returns wins as a percentage of rounds, rounded to two decimals
// because `ranks.win_rate` is decimal(5,2). Rounding here rather than letting
// MySQL truncate keeps the Go and PHP values identical.
func WinRate(wins, rounds int64) float64 {
	if rounds <= 0 || wins <= 0 {
		return 0
	}
	return math.Round(float64(wins)/float64(rounds)*100*100) / 100
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
