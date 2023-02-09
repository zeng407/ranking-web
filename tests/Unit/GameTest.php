<?php

namespace Tests\Unit;

use App\Enums\PostAccessPolicy;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use App\Services\GameService;
use Tests\TestCase;

class GameTest extends TestCase
{

    public function test_index_public_post()
    {
        $post = $this->seedPost();

        $res = $this->get(route('api.public-post.index'));
        $res->assertOk();

        $json = $res->decodeResponseJson();
        $data = $json->json('data');
        $this->assertIsArray($data);
        $this->assertEquals(1, count($data));

        $this->assertEquals($post->serial, $data[0]['serial']);
        $this->assertEquals($post->title, $data[0]['title']);
        $this->assertEquals($post->description, $data[0]['description']);
        $this->assertNotNull($data[0]['image1']['url']);
        $this->assertNotNull($data[0]['image1']['title']);
        $this->assertNotNull($data[0]['image2']['url']);
        $this->assertNotNull($data[0]['image2']['title']);
    }

    public function test_create_game()
    {
        $post = $this->seedPost();

        $res = $this->post(route('api.game.create'), [
            'post_serial' => $post->serial,
            'element_count' => config('setting.post_min_element_count')
        ]);
        $res->assertOk();

        $serial = $res->json('game_serial');
        $this->assertDatabaseHas('games', [
            'serial' => $serial,
            'element_count' => config('setting.post_min_element_count')
        ]);
    }

    public function test_show_game()
    {
        $post = $this->seedPost();
        $game = $this->seedGame($post, 8);

        $res = $this->get(route('api.game.next-round', $game->serial));
        $res->assertOk();

        $this->assertEquals(1, $res->json('data.current_round'));
        $this->assertEquals(4, $res->json('data.of_round'));
        $this->assertEquals(2, count($res->json('data.elements')));
    }

    public function test_game_private()
    {
        $post = $this->seedPost();
        $game = $this->seedGame($post, 1);

        $post->post_policy->access_policy = PostAccessPolicy::PRIVATE;
        $post->post_policy->save();

        $res = $this->get(route('api.game.next-round', $game->serial));
        \Log::debug($res->content());
        $res->assertStatus(403);
    }

    public function test_vote_game_8()
    {
        $post = $this->seedPost();
        $game = $this->seedGame($post, 8);

        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 4,
            'remain_elements' => 7
        ]);

        $this->vote($game, $log, [
            'current_round' => 2,
            'of_round' => 4,
            'remain_elements' => 6
        ]);

        $this->vote($game, $log, [
            'current_round' => 3,
            'of_round' => 4,
            'remain_elements' => 5
        ]);

        $this->vote($game, $log, [
            'current_round' => 4,
            'of_round' => 4,
            'remain_elements' => 4
        ]);

        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 2,
            'remain_elements' => 3
        ]);

        $this->vote($game, $log, [
            'current_round' => 2,
            'of_round' => 2,
            'remain_elements' => 2
        ]);

        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 1,
            'remain_elements' => 1
        ]);
    }

    public function test_vote_game_5()
    {
        $post = $this->seedPost();
        $game = $this->seedGame($post, 5);

        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 3,
            'remain_elements' => 4
        ]);

        $this->vote($game, $log, [
            'current_round' => 2,
            'of_round' => 3,
            'remain_elements' => 3
        ]);

        $this->vote($game, $log, [
            'current_round' => 3,
            'of_round' => 3,
            'remain_elements' => 2
        ]);

        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 1,
            'remain_elements' => 1
        ]);

    }

//    public function test_game_complete()
//    {
//        $post = $this->seedPost();
//        $game = $this->seedGame($post, 1);
//
//        $res = $this->get(route('api.game.show', $game->serial));
//        return $res->assertStatus(404);
//    }

    protected function vote(Game $game, &$log, $round)
    {
        if (!isset($log['winner'])) {
            $log['winner'] = [];
        }
        if (!isset($log['loser'])) {
            $log['loser'] = [];
        }

        $res = $this->get(route('api.game.next-round', $game->serial));
        $elements = $res->json('data.elements');
        $winner = $elements[0]['id'];
        $loser = $elements[1]['id'];

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
        $res = $this->post(route('api.game.vote', $data));
        $res->assertOk();

        $this->assertDatabaseHas('game_elements', [
            'game_id' => $game->id,
            'element_id' => $winner,
            'win_count' => $log['winner'][$winner],
            'is_eliminated' => false
        ]);
        $this->assertDatabaseHas('game_elements', [
            'game_id' => $game->id,
            'element_id' => $loser,
            'win_count' => $log['winner'][$loser] ?? 0,
            'is_eliminated' => true
        ]);
        $this->assertDatabaseHas('game_1v1_rounds', [
            'current_round' => $round['current_round'],
            'of_round' => $round['of_round'],
            'remain_elements' => $round['remain_elements'],
            'winner_id' => $winner,
            'loser_id' => $loser,
        ]);
    }

    protected function seedPost(): Post
    {
        /** @var User $user */
        $user = User::factory()->has(
            Post::factory()->has(
                PostPolicy::factory()->public(), 'post_policy'
            )
        )->create();

        $user->posts()->each(function (Post $post) {
            Element::factory(30)->hasAttached($post)->create();
        });

        return $user->posts()->first();
    }

    protected function seedGame(Post $post, $elementCount): Game
    {
        $game = app(GameService::class)->createGame($post, $elementCount);

        return $game;
    }
}
