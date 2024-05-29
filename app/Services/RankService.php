<?php


namespace App\Services;

use App\Enums\RankReportTimeRange;
use App\Enums\RankType;
use App\Jobs\UpdateRankReportHistory;
use App\Models\Element;
use App\Models\Game;
use App\Models\GameElement;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Models\RankReportHistory;
use App\Services\Builders\RankReprortHistoryBuilder;
use DB;

class RankService
{
    public function getRankReports(Post $post, $limit = 10, $page = null)
    {
        $reports = RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankPosition(Post $post, Element $element): ?int
    {
        $report = RankReport::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->first();

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
            ->where('rank', '>', 0)
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

    public function createElementRank(Game $game, Element $element)
    {
        logger("handle GameElementVoted $element->id");
        $post = $game->post;
        $completeGameRounds = GameElement::join('games', 'games.id', '=', 'game_elements.game_id')
            ->where('games.post_id', $post->id)
            ->where('game_elements.element_id', $element->id)
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
        $counter = 0;
        Rank::where('post_id', $post->id)
            ->where('rank_type', RankType::CHAMPION)
            ->where('record_date', today())
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->get()
            ->each(function (Rank $rank) use (&$counter) {
                $counter++;
                RankReport::updateOrCreate([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id
                ], [
                    'final_win_position' => $counter,
                    'final_win_rate' => $rank->win_rate,
                ]);
            });

        $counter = 0;
        Rank::where('post_id', $post->id)
            ->where('rank_type', RankType::PK_KING)
            ->where('record_date', today())
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->get()
            ->each(function (Rank $rank) use (&$counter) {
                $counter++;
                RankReport::updateOrCreate([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id
                ], [
                    'win_position' => $counter,
                    'win_rate' => $rank->win_rate,
                ]);
            });

        RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNotNull('deleted_at');
            })->delete();
        $counter = 0;
        RankReport::where('post_id', $post->id)
            ->orderByDesc('win_rate')
            ->orderByDesc('final_win_rate')
            ->get()
            ->each(function (RankReport $rankReport) use (&$counter) {
                $counter++;
                $rankReport->update([
                    'rank' => $counter
                ]);
            });
    }

    public function createRankReportHistory(RankReport $rankReport, RankReportTimeRange $timeRange, $refresh = false)
    {
        $builder = new RankReprortHistoryBuilder;
        return $builder->setRankReport($rankReport)
            ->setRange($timeRange)
            ->setRefresh($refresh)
            ->build();
    }

    public function updateRankReportHistoryRank(Post $post, RankReportTimeRange $timeRange, $startDate = null)
    {
        $query = RankReportHistory::where('post_id', $post->id)
            ->where('time_range', $timeRange);
        if($startDate) {
            $$query->where('start_date', '>=', $startDate);
        }

        $dates = $query->select('start_date')
            ->distinct()
            ->get()
            ->pluck('start_date');

        foreach ($dates as $date) {
            UpdateRankReportHistory::dispatch($post->id, $timeRange, $date);
        }
    }
}
