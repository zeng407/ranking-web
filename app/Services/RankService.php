<?php


namespace App\Services;


use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use DB;

class RankService
{
    public function getRankReports(Post $post, $limit = 10)
    {
        $reports = RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderBy('rank')
            ->paginate($limit);

        return $reports;
    }

    public function getRankPosition(Post $post, Element $element): ?int
    {
        $report = RankReport::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->first();

        return $report?->rank;
    }

    public function createElementRank(Game $game, Element $element)
    {
        \Log::debug("handle GameElementVoted $element->id");
        $post = $game->post;
        $topRankCounts = Game::where('games.post_id', $post->id)
            ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                $query->where('remain_elements', 1);
            })
            ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                $query->where('winner_id', $element->id)
                    ->orWhere('loser_id', $element->id);
            })
            ->count();

        if ($topRankCounts) {
            $winCount = Game::where('games.post_id', $post->id)
                ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                    $query->where('remain_elements', 1)
                        ->where('winner_id', $element->id);
                })
                ->count();

            if ($winCount) {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::CHAMPION,
                    'record_date' => today(),
                ], [
                    'win_count' => $winCount,
                    'round_count' => $topRankCounts,
                    'win_rate' => $winCount / $topRankCounts * 100
                ]);
            }
        }

        $pkCounts = Game::where('games.post_id', $post->id)
            ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where(function ($query) use ($element) {
                $query->where('winner_id', $element->id)
                    ->orWhere('loser_id', $element->id);
            })
            ->count();

        if ($pkCounts) {
            $winCount = Game::where('games.post_id', $post->id)
                ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
                ->where('winner_id', $element->id)
                ->count();

            if ($winCount) {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => $winCount,
                    'round_count' => $pkCounts,
                    'win_rate' => $winCount / $pkCounts * 100
                ]);
            } else {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => 0,
                    'round_count' => $pkCounts,
                    'win_rate' => 0
                ]);
            }
        }

    }

    public function createRankReport(Post $post)
    {
        \Log::info("update post [{$post->id}] CHAMPION rank");
        Rank::where('post_id', $post->id)
            ->where('rank_type', RankType::CHAMPION)
            ->where('record_date', today())
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->eachById(function (Rank $rank, $count) {
                RankReport::updateOrCreate([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id
                ], [
                    'final_win_position' => $count + 1,
                    'final_win_rate' => $rank->win_rate,
                ]);
            });

        \Log::info("update post [{$post->id}] PK_KING rank");
        Rank::where('post_id', $post->id)
            ->where('rank_type', RankType::PK_KING)
            ->where('record_date', today())
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->eachById(function (Rank $rank, $count) {
                RankReport::updateOrCreate([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id
                ], [
                    'win_position' => $count + 1,
                    'win_rate' => $rank->win_rate,
                ]);
            });

        \Log::info("update post [{$post->id}] rank report");
        RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('final_win_rate')
            ->orderByDesc('win_rate')
            ->eachById(function (RankReport $rankReport, $index) use ($post) {
                $rankReport->update([
                    'rank' => $index + 1
                ]);
            });
    }
}
