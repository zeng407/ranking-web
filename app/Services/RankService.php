<?php


namespace App\Services;


use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game;
use App\Models\GameElement;
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

    public function createRankReport(Post $post)
    {
        \Log::info("start update post [{$post->id}] rank report..."); 
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

        
        RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByDesc('win_rate')
            ->orderByDesc('final_win_rate')
            ->eachById(function (RankReport $rankReport, $index) use ($post) {
                $rankReport->update([
                    'rank' => $index + 1
                ]);
            });

        \Log::info("end update post [{$post->id}] rank report");
    }
}
