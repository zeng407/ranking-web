<?php

namespace Tests;

use App\Models\Game;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use App\Models\Element;
use App\Services\GameService;


trait TestHelper
{
    public function createPost(): Post
    {
        return Post::factory()->has(
            PostPolicy::factory()->public(),
            'post_policy'
        )->for(User::factory()->create())->create();
    }

    public function createElements(Post $post, $number = 1)
    {
        return Element::factory($number)->hasAttached($post)->create();
    }

    public function seedPost($elementsNumber = 16): Post
    {
        /** @var User $user */
        $user = User::factory()->has(
            Post::factory()->has(
                PostPolicy::factory()->public(), 'post_policy'
            )
        )->create();

        $user->posts()->each(function (Post $post) use ($elementsNumber) {
            Element::factory($elementsNumber)->hasAttached($post)->create();
        });

        return $user->posts()->first();
    }

    public function createGame(Post $post, $elementCount): Game
    {
        $game = app(GameService::class)->createGame($post, $elementCount);

        return $game;
    }

    public function voteGame(Game $game, &$log)
    {
        if (!isset($log['winner'])) {
            $log['winner'] = [];
        }
        if (!isset($log['loser'])) {
            $log['loser'] = [];
        }

        $res = $this->get(route('api.game.next-round', $game->serial));
        $elements = $res->json('data.elements');
        $winner = $elements[0]['id'] ?? null;
        $loser = $elements[1]['id'] ?? null;

        if (!isset($log['winner'][$winner])) {
            $log['winner'][$winner] = 0;
        }
        if (!isset($log['loser'][$loser])) {
            $log['loser'][$loser] = 0;
        }

        $log['winner'][$winner]++;
        $log['loser'][$loser]++;
        $data = [
            'game_serial' => $game->serial,
            'winner_id' => $winner,
            'loser_id' => $loser,
        ];
        session(['key' => 'value']);
        logger($data);
        return [
            'res' => $this->post(route('api.game.vote', $data)),
            'winner' => $winner,
            'loser' => $loser,
        ];
    }
}