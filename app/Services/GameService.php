<?php


namespace App\Services;


use App\Models\Element;
use App\Models\Game;
use App\Models\Post;

class GameService
{
    public function takeGameElements(Game $game, $count = 2)
    {
        $elements = $game->elements()
            ->wherePivot('is_eliminated', false)
            ->orderByPivot('win_count')
            ->inRandomOrder()
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
}
