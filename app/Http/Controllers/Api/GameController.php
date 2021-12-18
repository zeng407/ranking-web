<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\GameRoundResource;
use App\Http\Resources\GameResultResource;
use App\Http\Resources\GameSettingResource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Service\GameService;
use Illuminate\Http\Request;
use Illuminate\Support\Str;

class GameController extends Controller
{
    const END_GAME = 'end_game';
    const PROCESSING = 'processing';

    protected $gameService;

    public function __construct(GameService $gameService)
    {
        $this->gameService = $gameService;
    }

    public function getSetting($serial)
    {
        $post = $this->getPost($serial);

        return GameSettingResource::make($post);
    }

    public function nextRound($serial)
    {
        /** @var Game $game */
        $game = $this->getGame($serial);

        if (!$this->gameService->isGamePublic($game)) {
            return response()->json([
                'msg' => 'the game is private'
            ], 401);
        }

        // check game is complete
        if ($this->gameService->isGameComplete($game)) {
            return response()->json([
                'msg' => 'the game has completed'
            ], 404);
        }

        $elements = $this->gameService->takeGameElements($game, 2);

        return GameRoundResource::make($game, $elements);
    }

    public function create(Request $request)
    {
        $request->validate([
            'post_serial' => 'required',
            'element_count' => 'required|integer|min:4'
        ]);

        $post = $this->getPost($request->post_serial);

        $game = $this->gameService->createGame($post, $request->element_count);

        return response()->json([
            'game_serial' => $game->serial
        ]);
    }

    public function vote(Request $request)
    {
        $request->validate([
            'game_serial' => 'required',
            'winner_id' => 'required',
            'loser_id' => 'required',
        ]);

        //todo validate element id

        /** @var Game $game */
        $game = $this->getGame($request->game_serial);

        if (!$this->gameService->isGamePublic($game)) {
            return response()->json([
                'msg' => 'the game is private'
            ], 401);
        }

        // check game is complete
        if ($this->gameService->isGameComplete($game)) {
            return response()->json([
                'msg' => 'the game has completed'
            ], 404);
        }

        /** @var Game1V1Round $lastRound */
        $lastRound = $game->game_1v1_rounds()->latest('id')->first();
        if ($lastRound === null) {
            $round = 1;
            $ofRound = ceil($game->element_count / 2);
            $remain = $game->element_count - 1;
        } else if ($lastRound->current_round + 1 > $lastRound->of_round) {
            $round = 1;
            $ofRound = ceil($lastRound->remain_elements / 2);
            $remain = $lastRound->remain_elements - 1;
        } else {
            $round = $lastRound->current_round + 1;
            $ofRound = $lastRound->of_round;
            $remain = $lastRound->remain_elements - 1;
        }
        $data = [
            'current_round' => $round,
            'of_round' => $ofRound,
            'remain_elements' => $remain,
            'winner_id' => $request->winner_id,
            'loser_id' => $request->loser_id,
            'complete_at' => now(),
        ];
        \Log::info('saving game : ' . $game->serial, $data);
        $game->game_1v1_rounds()->create($data);

        // update winner
        $game->elements()
            ->updateExistingPivot($request->winner_id, [
                'win_count' => \DB::raw('win_count + 1')
            ]);

        // update loser
        $game->elements()
            ->updateExistingPivot($request->loser_id, [
                'is_eliminated' => true
            ]);

        return response()->json([
            'status' => $this->getStatus($game)
        ]);

    }

    protected function getStatus(Game $game)
    {
        if ($this->gameService->isGameComplete($game)) {
            return self::END_GAME;
        } else {
            return self::PROCESSING;
        }
    }

    protected function getGame($serial): Game
    {
        /** @var Game $game */
        $game = Game::where('serial', $serial)->firstOrFail();

        return $game;
    }

    protected function getPost($serial): Post
    {
        /** @var Post $post */
        $post = Post::where('serial', $serial)
            ->public()
            ->firstOrFail();

        return $post;
    }

}
