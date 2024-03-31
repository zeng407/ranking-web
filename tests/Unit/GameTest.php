<?php

namespace Tests\Unit;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Api\GameController;
use App\Models\Game;
use Tests\TestCase;
use Tests\TestHelper;

class GameTest extends TestCase
{
    use TestHelper;

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
        $game = $this->createGame($post, 8);

        $res = $this->get(route('api.game.next-round', $game->serial));
        $res->assertOk();

        $this->assertEquals(1, $res->json('data.current_round'));
        $this->assertEquals(4, $res->json('data.of_round'));
        $this->assertEquals(2, count($res->json('data.elements')));
    }

    public function test_game_private()
    {
        $post = $this->seedPost();
        $game = $this->createGame($post, 1);

        $post->post_policy->access_policy = PostAccessPolicy::PRIVATE;
        $post->post_policy->save();

        $res = $this->get(route('api.game.next-round', $game->serial));
        logger($res->content());
        $res->assertStatus(403);
    }

    public function test_vote_game_8()
    {
        $post = $this->seedPost();
        $game = $this->createGame($post, 8);

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

        session(['key' => 'value']);
        $this->vote($game, $log, [
            'current_round' => 1,
            'of_round' => 1,
            'remain_elements' => 1
        ]);
    }

    public function test_vote_game_5()
    {
        $post = $this->seedPost();
        $game = $this->createGame($post, 5);

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

    // public function testUpdateGameRoundsDeadlock()
    // {
    //     $post = $this->seedPost(8);
    //     $games = [
    //         $this->createGame($post, 8),
    //     ];
    //     $pids = [];
    //     //use file as cache driver
    //     config(['cache.default' => 'file']);
    //     for ($i = 0; $i < 5; $i++) {
            
    //         $pid = pcntl_fork();
    //         if ($pid == -1) {
    //             throw new \Exception('Could not fork');
    //         } else if ($pid) {
    //             // In the parent process
    //             $pids[] = $pid;
    //         } else {
    //             // In the child process
    //             $elements = 31;
    //             while ($elements--) {
    //                 try{
    //                     $this->vote($games[0], $log);
    //                     // avoid middleware 'throttle:api', when testing
    //                     \Carbon\Carbon::setTestNow(Carbon::now()->addSeconds(3));
    //                 }catch (\Exception $e){
    //                     report($e);
    //                     exit(1);
    //                 }
    //             }
    //             exit(0);
    //         }
    //     }

    //     foreach ($pids as $pid) {
    //         // Wait for the child processes to finish
    //         pcntl_waitpid($pid, $status);
    //     }

    //     if(isset($log)){
    //         dump($log);
    //     }
    //     // $this->assertDatabaseHas('game_1v1_rounds', [
    //     //     'current_round' => 1,
    //     //     'of_round' => 1,
    //     //     'remain_elements' => 1,
    //     // ]);
    // }

    protected function vote(Game $game, &$log, $assert = [])
    {
        $result = $this->voteGame($game, $log);
        $result['res']->assertOk();
        if ($result['res']->json('status') != GameController::PROCESSING) {
            return;
        }

        $winner = $result['winner'];
        $loser = $result['loser'];
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
        if ($assert) {
            $this->assertDatabaseHas('game_1v1_rounds', [
                'current_round' => $assert['current_round'],
                'of_round' => $assert['of_round'],
                'remain_elements' => $assert['remain_elements'],
                'winner_id' => $winner,
                'loser_id' => $loser,
            ]);
        }

    }

}
