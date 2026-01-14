<?php

namespace App\Services\Builders;

use App\Enums\RankReportTimeRange;
use App\Enums\RankType;
use App\Helper\CacheService;
use App\Helper\Locker;
use App\Models\Element;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Models\RankReportHistory;

class RankReportHistoryBuilder
{
    protected RankReport $report;
    protected RankReportTimeRange $range;

    protected $refresh = false;
    protected $startAt = null;

    public function setStartAt($startAt): self
    {
        $this->startAt = $startAt;
        return $this;
    }

    public function setRankReport(RankReport $report): self
    {
        $this->report = $report;
        return $this;
    }

    public function setRange(RankReportTimeRange $range): self
    {
        $this->range = $range;
        return $this;
    }

    public function setRefresh(bool $refresh): self
    {
        $this->refresh = $refresh;
        return $this;
    }

    public function build()
    {
        switch ($this->range) {
            case RankReportTimeRange::WEEK:
                return $this->buildWeek();
            case RankReportTimeRange::MONTH:
                return $this->buildMonth();
            case RankReportTimeRange::YEAR:
                return $this->buildYear();
            case RankReportTimeRange::ALL:
                return $this->buildAll();
            case RankReportTimeRange::THOUSAND_VOTES:
                return $this->buildThousandVotes();
        }
    }

    protected function buildAll()
    {
        if ($this->refresh) {
            $this->deleteHistory(RankReportTimeRange::ALL);
        }

        $start = $this->getLastStartDate(RankReportTimeRange::ALL);

        $lastRecord = $this->getLastRankRecord($start, RankType::PK_KING);
        $sumWinCount = $lastRecord->win_count ?? 0;
        $sumLoseCount = $lastRecord ? $lastRecord->round_count - $lastRecord->win_count : 0;
        $sumRounds = $lastRecord->round_count ?? 0;
        if ($lastRecord) {
            $start = carbon($lastRecord->record_date)->toDateString();
        }
        // skip if no one played the game in these days
        if ($sumRounds == 0) {
            return;
        }

        $lastRecord = $this->getLastRankRecord($start, RankType::CHAMPION);
        $championCount = $lastRecord->win_count ?? 0;
        $gameCompleteCount = $lastRecord->round_count ?? 0;

        $rankRecords = $this->getRankRecords($start);
        $timeline = carbon($start);
        while ($timeline->lt(today())) {
            $rankPKRecord = $rankRecords->where('record_date', carbon($timeline)->toDateString())
                ->where('rank_type', RankType::PK_KING)
                ->first();
            if ($rankPKRecord) {
                $sumWinCount = $rankPKRecord->win_count;
                $sumLoseCount = $rankPKRecord->round_count - $rankPKRecord->win_count;
                $sumRounds = $rankPKRecord->round_count;
            }

            $rankChampionRecord = $rankRecords->where('record_date', carbon($timeline)->toDateString())
                ->where('rank_type', RankType::CHAMPION)
                ->first();
            if ($rankChampionRecord) {
                $championCount = $rankChampionRecord->win_count;
                $gameCompleteCount = $rankChampionRecord->round_count;
            }

            $winRate = $sumRounds > 0 ? $sumWinCount / $sumRounds * 100 : 0;
            $championRate = $gameCompleteCount > 0 ? $championCount / $gameCompleteCount * 100 : 0;
            $existsReport = RankReportHistory::where('element_id', $this->report->element_id)
                ->where('post_id', $this->report->post_id)
                ->where('rank_report_id', $this->report->id)
                ->where('time_range', RankReportTimeRange::ALL)
                ->where('start_date', $timeline->toDateString())
                ->exists();
            if ($sumWinCount > 0 && !$existsReport) {
                RankReportHistory::updateOrCreate([
                    'element_id' => $this->report->element_id,
                    'post_id' => $this->report->post_id,
                    'rank_report_id' => $this->report->id,
                    'time_range' => RankReportTimeRange::ALL,
                    'start_date' => $timeline->toDateString(),
                ], [
                    'rank' => 0, // we mark the rank as 0, then update the rank later
                    'win_rate' => $winRate,
                    'win_count' => $sumWinCount,
                    'lose_count' => $sumLoseCount,
                    'champion_count' => $championCount,
                    'game_complete_count' => $gameCompleteCount,
                    'champion_rate' => $championRate
                ]);

                try {
                    $locker = Locker::lockRankHistory($this->report->post_id);
                    $locker->block(5);
                    CacheService::putRankHistoryNeededUpdateDatesCache(
                        $this->report->post_id,
                        RankReportTimeRange::ALL,
                        $timeline->toDateString()
                    );
                    $locker->release();
                } catch (\Exception $e) {
                    report($e);
                    $locker->release();
                }
            }

            $timeline->addDay();
        }
    }

    protected function buildWeek()
    {
        if ($this->refresh) {
            $this->deleteHistory(RankReportTimeRange::WEEK);
        }

        $start = $this->getLastStartDate(RankReportTimeRange::WEEK);

        $rankRecords = $this->getRankRecords(carbon($start)->startOfWeek());

        $lastChampionRecord = $this->getLastWeeklyRankRecord($start, RankType::CHAMPION);
        $lastChampionCount = $lastChampionRecord->win_count ?? 0;
        $lastGameCompleteCount = $lastChampionRecord->round_count ?? 0;

        $lastPKRecord = $this->getLastWeeklyRankRecord($start, RankType::PK_KING);
        $lastWinCount = $lastPKRecord->win_count ?? 0;
        $lastLoseCount = $lastPKRecord ? $lastPKRecord->round_count - $lastPKRecord->win_count : 0;
        $lastRounds = $lastPKRecord->round_count ?? 0;

        $timeline = carbon($start)->endOfWeek();
        $endOfWeek = carbon(today())->endOfWeek();
        while ($timeline->lte($endOfWeek)) {
            $rankPKRecord = $rankRecords
                ->where('record_date', '<=', carbon($timeline)->endOfWeek()->toDateString())
                ->where('rank_type', RankType::PK_KING)
                ->last();
            $rankChampionRecord = $rankRecords
                ->where('record_date', '<=', carbon($timeline)->endOfWeek()->toDateString())
                ->where('rank_type', RankType::CHAMPION)
                ->last();
            $sumRounds = $rankPKRecord ? $rankPKRecord->round_count - $lastRounds : 0;

            $sumWinCount = $rankPKRecord ? $rankPKRecord->win_count - $lastWinCount : 0;
            $sumLoseCount = $rankPKRecord ? $rankPKRecord->round_count - $rankPKRecord->win_count - $lastLoseCount : 0;
            $championCount = $rankChampionRecord ? $rankChampionRecord->win_count - $lastChampionCount : 0;
            $gameCompleteCount = $rankChampionRecord ? $rankChampionRecord->round_count - $lastGameCompleteCount : 0;
            $winRate = $sumRounds > 0 ? $sumWinCount / $sumRounds * 100 : 0;
            $championRate = $gameCompleteCount > 0 ? $championCount / $gameCompleteCount * 100 : 0;

            $existsReport = RankReportHistory::where('element_id', $this->report->element_id)
                ->where('post_id', $this->report->post_id)
                ->where('rank_report_id', $this->report->id)
                ->where('time_range', RankReportTimeRange::WEEK)
                ->where('start_date', $timeline->toDateString())
                ->exists();
            $skipExists = $existsReport && $timeline->lt(today()->subDays(2)->startOfWeek());

            if ($sumWinCount > 0 && !$skipExists) {
                RankReportHistory::updateOrCreate([
                    'element_id' => $this->report->element_id,
                    'post_id' => $this->report->post_id,
                    'rank_report_id' => $this->report->id,
                    'time_range' => RankReportTimeRange::WEEK,
                    'start_date' => $timeline->toDateString(),
                ], [
                    'rank' => 0, // we mark the rank as 0, then update the rank later
                    'win_rate' => $winRate,
                    'win_count' => $sumWinCount,
                    'lose_count' => $sumLoseCount,
                    'champion_count' => $championCount,
                    'game_complete_count' => $gameCompleteCount,
                    'champion_rate' => $championRate
                ]);

                try {
                    $locker = Locker::lockRankHistory($this->report->post_id);
                    $locker->block(5);
                    CacheService::putRankHistoryNeededUpdateDatesCache(
                        $this->report->post_id,
                        RankReportTimeRange::WEEK,
                        $timeline->toDateString()
                    );
                    $locker->release();
                } catch (\Exception $e) {
                    report($e);
                    $locker->release();
                }
            }

            $lastChampionCount = $rankChampionRecord ? $rankChampionRecord->win_count : 0;
            $lastGameCompleteCount = $rankChampionRecord ? $rankChampionRecord->round_count : 0;
            $lastWinCount = $rankPKRecord ? $rankPKRecord->win_count : 0;
            $lastLoseCount = $rankPKRecord ? $rankPKRecord->round_count - $rankPKRecord->win_count : 0;
            $lastRounds = $rankPKRecord ? $rankPKRecord->round_count : 0;

            $timeline->addWeek();
        }
    }


    protected function buildThousandVotes()
    {
        if ($this->refresh) {
            $this->deleteHistory(RankReportTimeRange::THOUSAND_VOTES);
            $this->deleteThousandVotesCache();
        }

        if ($this->isThousandVotesUpdatedToday() && !$this->refresh) {
            return;
        }

        $cachedSummary = $this->getThousandVotesCachedIds();

        if (empty($cachedSummary)) {
            $this->buildThousandVotesFromScratch();
        } else {
            $this->updateThousandVotesIncremental($cachedSummary);
        }
    }

    protected function isThousandVotesUpdatedToday()
    {
        return RankReportHistory::where('element_id', $this->report->element_id)
            ->where('post_id', $this->report->post_id)
            ->where('rank_report_id', $this->report->id)
            ->where('time_range', RankReportTimeRange::THOUSAND_VOTES)
            ->where('start_date', today()->toDateString())
            ->exists();
    }

    protected function getThousandVotesBaseQuery()
    {
        return Game1V1Round::where(function ($query) {
                $query->where('winner_id', $this->report->element_id)
                      ->orWhere('loser_id', $this->report->element_id);
            })
            ->join('games', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $this->report->post_id)
            ->select('game_1v1_rounds.id', 'game_1v1_rounds.winner_id', 'game_1v1_rounds.loser_id');
    }

    protected function buildThousandVotesFromScratch()
    {
        $latestVotes = $this->getThousandVotesBaseQuery()
            ->orderByDesc('game_1v1_rounds.id')
            ->limit(1000)
            ->get();

        if ($latestVotes->isEmpty()) {
            return;
        }

        $winCount = $latestVotes->where('winner_id', $this->report->element_id)->count();
        $loseCount = $latestVotes->where('loser_id', $this->report->element_id)->count();

        $maxId = $latestVotes->first()->id;
        $minId = $latestVotes->last()->id;

        $this->saveThousandVotesHistory($winCount, $loseCount, $maxId, $minId);
    }

    protected function updateThousandVotesIncremental(array $cachedSummary)
    {
        $winCount = $cachedSummary['winner_count'] ?? 0;
        $loseCount = $cachedSummary['loser_count'] ?? 0;
        $maxId = $cachedSummary['max_id'] ?? 0;
        $minId = $cachedSummary['min_id'] ?? 0;

        // Find new votes after cached max_id (up to 1000)
        $newVotes = $this->getThousandVotesBaseQuery()
            ->where('game_1v1_rounds.id', '>', $maxId)
            ->orderByDesc('game_1v1_rounds.id')
            ->limit(1000)
            ->get();

        $newCount = $newVotes->count();
        if ($newCount == 0) {
            return;
        }

        $winNew = $newVotes->where('winner_id', $this->report->element_id)->count();
        $loseNew = $newVotes->where('loser_id', $this->report->element_id)->count();

        $winOutdated = 0;
        $loseOutdated = 0;
        $newMinId = $minId;

        if ($newCount > 0 && $minId > 0) {
            // Fetch the oldest portion plus the next one to derive newMinId
            $outdatedVotes = $this->getThousandVotesBaseQuery()
                ->where('game_1v1_rounds.id', '>=', $minId)
                ->orderBy('game_1v1_rounds.id')
                ->limit($newCount + 1)
                ->get();

            if ($outdatedVotes->isNotEmpty()) {
                $outdatedSlice = $outdatedVotes->take($newCount);
                $winOutdated = $outdatedSlice->where('winner_id', $this->report->element_id)->count();
                $loseOutdated = $outdatedSlice->where('loser_id', $this->report->element_id)->count();

                $newMinId = $outdatedVotes->last()->id;
            }
        }

        $winCount = max(0, $winCount - $winOutdated + $winNew);
        $loseCount = max(0, $loseCount - $loseOutdated + $loseNew);
        $maxId = $newVotes->isNotEmpty() ? max($maxId, $newVotes->first()->id) : $maxId;
        $minId = $newMinId ?: $minId;

        $this->saveThousandVotesHistory($winCount, $loseCount, $maxId, $minId);
    }

    protected function saveThousandVotesHistory($winCount, $loseCount, $maxId, $minId)
    {
        $today = today()->toDateString();
        $totalRounds = $winCount + $loseCount;
        $winRate = $totalRounds > 0 ? ($winCount / $totalRounds * 100) : 0;

        RankReportHistory::updateOrCreate(
            [
                'element_id' => $this->report->element_id,
                'post_id' => $this->report->post_id,
                'rank_report_id' => $this->report->id,
                'time_range' => RankReportTimeRange::THOUSAND_VOTES,
                'start_date' => $today,
            ],
            [
                'rank' => 0,
                'win_rate' => $winRate,
                'win_count' => $winCount,
                'lose_count' => $loseCount,
                'champion_count' => 0,
                'game_complete_count' => 0,
                'champion_rate' => 0,
            ]
        );

        $this->putThousandVotesCachedIds([
            'max_id' => $maxId,
            'min_id' => $minId,
            'winner_count' => $winCount,
            'loser_count' => $loseCount,
        ]);

        try {
            $locker = Locker::lockRankHistory($this->report->post_id);
            $locker->block(5);
            CacheService::putRankHistoryNeededUpdateDatesCache(
                $this->report->post_id,
                RankReportTimeRange::THOUSAND_VOTES,
                $today
            );
            $locker->release();
        } catch (\Exception $e) {
            report($e);
            $locker->release();
        }
    }

    protected function buildMonth()
    {
    }

    protected function buildYear()
    {
    }

    protected function getLastStartDate(RankReportTimeRange $range)
    {
        $lastReport = RankReportHistory::where('rank_report_id', $this->report->id)
            ->where('time_range', $range)
            ->orderByDesc('start_date')
            ->first();
        if ($lastReport) {
            $start = $lastReport->start_date;
        } else if ($this->startAt) {
            $start = carbon($this->startAt)->toDateString();
        } else {
            $start = carbon($this->report->post->created_at)->toDateString();
        }

        return $start;
    }

    protected function deleteHistory(RankReportTimeRange $range)
    {
        RankReportHistory::where('rank_report_id', $this->report->id)
            ->where('time_range', $range)
            ->delete();
    }

    protected function getRankRecords($startDate)
    {
        $gameRecords = Rank::where('post_id', $this->report->post_id)
            ->where('element_id', $this->report->element_id)
            ->where('record_date', '>=', $startDate)
            ->orderBy('record_date')
            ->get();

        return $gameRecords;
    }

    protected function getLastRankRecord($afterDate, $rankType = null)
    {
        $lastRecord = Rank::where('post_id', $this->report->post_id)
            ->where('element_id', $this->report->element_id)
            ->when($rankType, function ($query) use ($rankType) {
                $query->where('rank_type', $rankType);
            })
            ->where('record_date', '>=', $afterDate)
            ->orderBy('record_date')
            ->first();

        return $lastRecord;
    }

    protected function getLastWeeklyRankRecord($afterDate, $rankType = null)
    {
        $lastRecord = Rank::where('post_id', $this->report->post_id)
            ->where('element_id', $this->report->element_id)
            ->when($rankType, function ($query) use ($rankType) {
                $query->where('rank_type', $rankType);
            })
            ->where('record_date', '>=', carbon($afterDate)->startOfWeek())
            ->orderBy('record_date')
            ->first();

        return $lastRecord;
    }

    /**
     * Get cached summary for thousand votes: ['max_id', 'min_id', 'winner_count', 'loser_count']
     * @return array
     */
    protected function getThousandVotesCachedIds()
    {
        return CacheService::getThousandVotesCachedIds($this->report->post_id, $this->report->element_id);
    }

    /**
     * Store cached summary for thousand votes
     * @param array $summary ['max_id', 'min_id', 'winner_count', 'loser_count']
     */
    protected function putThousandVotesCachedIds($summary)
    {
        CacheService::putThousandVotesCachedIds($this->report->post_id, $this->report->element_id, $summary);
    }

    /**
     * Delete cached thousand votes when refreshing
     */
    protected function deleteThousandVotesCache()
    {
        CacheService::deleteThousandVotesCache($this->report->post_id, $this->report->element_id);
    }
}
