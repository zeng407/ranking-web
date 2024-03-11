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
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;

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
            'element_count' => ['required', 'integer', 'min:' . config('setting.post_min_element_count'),
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
            'winner_id' => 'required',
            'loser_id' => 'required',
        ]);
        /** @var Game $game */
        $game = $this->getGame($request->game_serial);

        /**
         * @see GamePolicy::play()
         */
        $this->authorize('play', $game);

        /** @var Game1V1Round $lastRound */
        $lastRound = $game->game_1v1_rounds()->latest('id')->first();
        if ($lastRound === null) {
            $round = 1;
            $ofRound = (int)ceil($game->element_count / 2);
            $remain = $game->element_count - 1;
        } else if ($lastRound->current_round + 1 > $lastRound->of_round) {
            $round = 1;
            $ofRound = (int)ceil($lastRound->remain_elements / 2);
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
        /** @var Game1V1Round $gameRound */
        $gameRound = $game->game_1v1_rounds()->create($data);

        // update winner
        $game->elements()
            ->wherePivot('is_eliminated', false)
            ->updateExistingPivot($request->winner_id, [
                'win_count' => \DB::raw('win_count + 1')
            ]);

        // update loser
        $game->elements()
            ->wherePivot('is_eliminated', false)
            ->updateExistingPivot($request->loser_id, [
                'is_eliminated' => true
            ]);

        event(new GameElementVoted($game, $gameRound->winner));
        event(new GameElementVoted($game, $gameRound->loser));

        // update rank when game complete
        if ($this->gameService->isGameComplete($game)) {
            event(new GameComplete($game));
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

    public function result(Game $game)
    {
        if (!$this->gameService->isGameComplete($game)) {
            return response()->json([], 404);
        }

        $rounds = $game->game_1v1_rounds()
            ->orderBy('remain_elements')
            ->take(9)
            ->get();

        $winner = $this->gameService->getWinner($game);

        return GameResultResource::collection($rounds)
            ->additional([
                'winner' => $winner,
                'winner_rank' => $this->rankService->getRankPosition($game->post, $winner)
            ]);
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
