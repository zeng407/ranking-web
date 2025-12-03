<?php


namespace App\Services;

use App\Enums\RankReportTimeRange;
use App\Enums\RankType;
use App\Helper\CacheService;
use App\Jobs\UpdateRankForReportHistory;
use App\Models\Element;
use App\Models\Game;
use App\Models\GameElement;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Models\RankReportHistory;
use App\Services\Builders\RankReportHistoryBuilder;
use DB;

class RankService
{
    public function getRankReports(Post $post, $limit = 10, $page = null)
    {
        $reports = RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByRaw('ISNULL(`rank`)')
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankReportByElement(Post $post, Element $element): ?RankReport
    {
        return RankReport::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->first();
    }

    public function getRanksByElementTitle(Post $post, string $title)
    {
        return RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) use ($title) {
                $query->where('title', 'like', '%' . $title . '%');
            })
            ->limit(10)
            ->orderBy('rank')
            ->get();
    }

    public function getRankPosition(Post $post, Element $element): ?int
    {
        $report = $this->getRankReportByElement($post, $element);

        return $report?->rank;
    }

    public function getRankWeeklyReport(Post $post, $limit = 10, $page = null)
    {
        $latestDate = RankReportHistory::where('post_id', $post->id)
            ->where('time_range', RankReportTimeRange::WEEK)
            ->max('start_date');
        $reports = $post->rank_report_histories()->with('element')
            ->where('time_range', RankReportTimeRange::WEEK)
            ->where('start_date', $latestDate)
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankReportHistoryByRankReport(RankReport $rankReport, RankReportTimeRange $timeRange, $limit = 10, $page = null)
    {
        $reports = $rankReport->rank_report_histories()
            ->where('time_range', $timeRange)
            ->orderByDesc('start_date')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankReportHistoryByElement(Post $post, Element $element, RankReportTimeRange $timeRange, $limit = 10, $page = null)
    {
        $reports = RankReportHistory::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->where('time_range', $timeRange)
            ->orderByDesc('start_date')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function createElementRank(Post $post, Element $element)
    {
        $completeGameRounds = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->where(function($subQuery) use ($element) {
                $subQuery->where('game_1v1_rounds.winner_id', $element->id)
                    ->orWhere('game_1v1_rounds.loser_id', $element->id);
            })
            ->whereNotNull('games.completed_at')
            ->count();

        if ($completeGameRounds) {
            $championCount = Game::where('games.post_id', $post->id)
                ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                    $query->where('remain_elements', 1)
                        ->where('winner_id', $element->id);
                })
                ->count();

            if ($championCount) {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::CHAMPION,
                    'record_date' => today(),
                ], [
                    'win_count' => $championCount,
                    'round_count' => $completeGameRounds,
                    'win_rate' => $championCount / $completeGameRounds * 100
                ]);
            }
        }

        $winCount = Game::where('games.post_id', $post->id)
            ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('game_1v1_rounds.winner_id', $element->id)
            ->count();
        $loseCount = Game::where('games.post_id', $post->id)
            ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('game_1v1_rounds.loser_id', $element->id)
            ->count();

        $rounds = $winCount + $loseCount;
        if ($rounds > 0) {
            if ($winCount) {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => $winCount,
                    'round_count' => $rounds,
                    'win_rate' => $winCount / $rounds * 100
                ]);
            } else {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => 0,
                    'round_count' => $rounds,
                    'win_rate' => 0
                ]);
            }
        }
    }

    public function createRankReports(Post $post)
    {
        \Log::info("start update post [{$post->id}] rank report [{$post->title}]");

        DB::transaction(function () use ($post) {
            $baseRankQuery = Rank::query()
                ->select(['post_id', 'element_id', 'rank_type', 'win_rate', 'win_count'])
                ->where('post_id', $post->id)
                ->where('record_date', today())
                ->whereHas('element', function ($query) {
                    $query->whereNull('deleted_at');
                });

            $rankReports = RankReport::where('post_id', $post->id)
                ->whereNull('deleted_at')
                ->get()
                ->keyBy('element_id');

            $championRanks = (clone $baseRankQuery)
                ->where('rank_type', RankType::CHAMPION)
                ->orderByDesc('win_rate')
                ->orderByDesc('win_count')
                ->get();
            $this->applyRankUpdates($championRanks, $rankReports, 'final_win_position', 'final_win_rate');

            $pkRanks = (clone $baseRankQuery)
                ->where('rank_type', RankType::PK_KING)
                ->orderByDesc('win_rate')
                ->orderByDesc('win_count')
                ->get();
            $this->applyRankUpdates($pkRanks, $rankReports, 'win_position', 'win_rate');

            RankReport::where('post_id', $post->id)
                ->whereHas('element', function ($query) {
                    $query->whereNotNull('deleted_at');
                })
                ->delete();

            $orderedIds = RankReport::where('post_id', $post->id)
                ->orderByDesc('win_rate')
                ->orderByDesc('final_win_rate')
                ->pluck('id');

            if ($orderedIds->isNotEmpty()) {
                $caseSql = 'CASE id';
                foreach ($orderedIds as $index => $id) {
                    $rank = $index + 1;
                    $caseSql .= " WHEN {$id} THEN {$rank}";
                }
                $caseSql .= ' END';

                DB::table('rank_reports')
                    ->whereIn('id', $orderedIds)
                    ->update(['rank' => DB::raw($caseSql)]);
            }
        });
        \Log::info("end update post [{$post->id}] rank report [{$post->title}]");
    }

    private function applyRankUpdates($ranks, $rankReports, string $positionField, string $rateField): void
    {
        $counter = 0;
        foreach ($ranks as $rank) {
            $counter++;
            $report = $rankReports->get($rank->element_id);

            if (!$report) {
                $report = new RankReport([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id,
                ]);
                $rankReports->put($rank->element_id, $report);
            }

            $report->setAttribute($positionField, $counter);
            $report->setAttribute($rateField, $rank->win_rate);
            $report->save();
        }
    }

    public function createRankReportHistory(RankReport $rankReport, RankReportTimeRange $timeRange, $refresh = false, $start = null)
    {
        $builder = new RankReportHistoryBuilder;
        return $builder->setRankReport($rankReport)
            ->setStartAt($start)
            ->setRange($timeRange)
            ->setRefresh($refresh)
            ->build();
    }

    public function updateRankReportHistoryRank(Post $post, RankReportTimeRange $timeRange)
    {
        $dates = CacheService::pullRankHistoryNeededUpdateDatesCache($post->id, $timeRange);
        if (empty($dates)) {
            return;
        }
        foreach ($dates as $date) {
            UpdateRankForReportHistory::dispatch($post->id, $timeRange, $date);
        }
    }
}
