<?php

namespace App\Http\Controllers\Api;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Http\Controllers\Controller;
use App\Http\Resources\Game\GameResultResource;
use App\Http\Resources\Game\GameRoundResource;
use App\Http\Resources\PublicPostResource;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Rules\GameElementRule;
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;
use Ramsey\Uuid\Uuid;

class GameController extends Controller
{
    const END_GAME = 'end_game';
    const PROCESSING = 'processing';

    protected GameService $gameService;
    protected RankService $rankService;

    public function __construct(GameService $gameService, RankService $rankService)
    {
        $this->gameService = $gameService;
        $this->rankService = $rankService;
    }

    public function getSetting(Post $post)
    {
        /** @see PostPolicy::publicRead() */
        $this->authorize('public-read', $post);

        return PublicPostResource::make($post);
    }

    public function nextRound(Game $game)
    {
        /** @see GamePolicy::play() */
        $this->authorize('play', $game);

        $elements = $this->gameService->takeGameElements($game, 2);

        return GameRoundResource::make($game, $elements);
    }

    public function create(Request $request)
    {
        $request->validate([
            'post_serial' => 'required',
            'element_count' => [
                'required',
                'integer',
                'min:' . config('setting.post_min_element_count'),
                'max:' . config('setting.post_max_element_count')
            ]
        ]);

        $post = $this->getPost($request->post_serial);
        /**
         * @see PostPolicy::newGame()
         */
        $this->authorize('new-game', $post);

        $game = $this->gameService->createGame($post, $request->element_count);

        return response()->json([
            'game_serial' => $game->serial
        ]);
    }

    public function vote(Request $request)
    {
        $request->validate([
            'game_serial' => 'required',
            'winner_id' => ['required'],
            'loser_id' => ['required'],
        ]);

        //todo lock transaction

        /** @var Game $game */
        $game = $this->getGame($request->game_serial);

        /**
         * @see GamePolicy::play()
         */
        $this->authorize('play', $game);

        //todo critical bug: if user vote illegal element, it will be error
        $gameRound = $this->gameService->updateGameRounds($game, $request->winner_id, $request->loser_id);

        event(new GameElementVoted($game, $gameRound->winner));
        event(new GameElementVoted($game, $gameRound->loser));

        // update rank when game complete
        if ($this->gameService->isGameComplete($game)) {
            $anonymousId = session()->get('anonymous_id', 'unknown');
            event(new GameComplete($request->user(), $anonymousId, $game, $gameRound->winner));
        }

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
        $post = Post::where('serial', $serial)->firstOrFail();

        return $post;
    }

}
