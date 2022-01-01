<?php


namespace App\Services;


use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;

class RankService
{
    public function takeGameElements(Game $game, $count = 2)
    {
        $elements = $game->elements()
            ->wherePivot('is_eliminated', false)
            ->orderByPivot('win_count')
            ->take($count)
            ->get();

        return $elements;
    }

    public function createGame(Post $post, $elementCount): Game
    {
        /** @var Game $game */
        $game = $post->games()->create([
            'serial' => \Str::random(8),
            'element_count' => $elementCount
        ]);

        // pick random elements
        $elements = $post->elements()
            ->inRandomOrder()
            ->take($game->element_count)
            ->get();

        $elements->each(function (Element $element) use ($game) {
            $game->elements()->attach($element);
        });

        return $game;
    }

    public function isGamePublic(Game $game)
    {
        return $game->post->isPublic();
    }

    public function isGameComplete(Game $game)
    {
        return $game->game_1v1_rounds()
            ->where('remain_elements', 1)
            ->exists();
    }

    public function createRankReport(Post $post)
    {
        $post->elements->each(function (Element $element) use ($post) {

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
                }
            }
        });

        Rank::where('post_id', $post->id)
            ->where('rank_type', RankType::CHAMPION)
            ->where('record_date', today())
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->eachById(function (Rank $rank, $count) {
                \Log::debug("update CHAMPION rank position [$count] {$rank->id}");
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
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->eachById(function (Rank $rank, $count) {
                \Log::debug("update PK_KING rank position [$count] {$rank->id}");
                RankReport::updateOrCreate([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id
                ], [
                    'win_position' => $count + 1,
                    'win_rate' => $rank->win_rate,
                ]);
            });
    }
}
