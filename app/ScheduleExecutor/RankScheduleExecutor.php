<?php


namespace App\ScheduleExecutor;


use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;

class RankScheduleExecutor
{
    public function createRankReport()
    {
        Post::eachById(function (Post $post, $count) {
            \Log::debug("post[$count] {$post->id} {$post->serial}");
            $post->elements->each(function (Element $element) use ($post) {

                $topRankCounts = Game::where('games.post_id', $post->id)
                    ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                        $query->where('current_round', 1)
                            ->where('of_round', 1)
                            ->where('remain_elements', 1);
                    })
                    ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                        $query->where('winner_id', $element->id)
                            ->orWhere('loser_id', $element->id);
                    })
                    ->count();

                if ($topRankCounts) {
                    $winCount = Game::where('games.post_id', $post->id)
                        ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                            $query->where('current_round', 1)
                                ->where('of_round', 1)
                                ->where('remain_elements', 1)
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
                    }
                }
            });
        });
    }
}
