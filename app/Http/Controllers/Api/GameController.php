<?php

namespace App\Http\Controllers\Api;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Helper\AccessTokenService;
use App\Http\Controllers\Controller;
use App\Http\Resources\Game\GameRoundResource;
use App\Http\Resources\PublicPostResource;
use App\Models\Game;
use App\Models\Post;
use App\Rules\GameCandicateRule;
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

    public function getSetting(Request $request, Post $post)
    {
        $request->validate([
           'password' => 'string|nullable'
        ]);

        /** @see \App\Policies\PostPolicy::publicRead() */
        $this->authorize('public-read', [$post, $request->header('Authorization')]);

        return PublicPostResource::make($post);
    }

    public function nextRound(Game $game)
    {
        /** @see \App\Policies\GamePolicy::play() */
        $this->authorize('play', $game);

        return $this->getNextElements($game);
    }

    public function create(Request $request)
    {
        $request->validate([
            'post_serial' => 'required',
            'element_count' => [
                'required',
                'integer',
                'min:2',
                'max:' . config('setting.post_max_element_count')
            ],
        ]);

        $post = $this->getPost($request->post_serial);

        /** @see \App\Policies\PostPolicy::newGame() */
        $this->authorize('new-game', [$post, $request->input('password')]);

        $game = $this->gameService->createGame($post, $request->element_count);

        $elements = $this->getNextElements($game);

        return response()->json([
            'game_serial' => $game->serial,
            'data' => $elements
        ]);
    }

    public function access(Request $request, Post $post)
    {
        // rate limit access for 10 times per minute
        $accessKey = 'access:' . $post->id;

        if (!\RateLimiter::tooManyAttempts($accessKey, 10)) {
            \RateLimiter::hit($accessKey, 60);
        } else {
            return response()->json([], 429);
        }
        logger('accessed', ['post' => $post->id]);
        if(
            $post->isPasswordRequired()
            && !empty($request->header('Authorization')
            && is_string($request->header('Authorization')))
        ){
            $hash = hash('sha256', $request->header('Authorization'));
            if($hash === $post->post_policy->password){
                AccessTokenService::setPostAccessToken($post, $hash);
                return response()->json();
            }
        }
        return response()->json([], 403);
    }

    public function vote(Request $request)
    {

        $request->validate([
            'game_serial' => 'required',
        ]);
        /** @var Game $game */
        $game = $this->getGame($request->input('game_serial'));
        $request->validate([
            'winner_id' => ['required', 'different:loser_id', new GameCandicateRule($game)],
            'loser_id' => ['required', 'different:winner_id', new GameCandicateRule($game)]
        ]);

        /** @see \App\Policies\GamePolicy::play() */
        $this->authorize('play', $game);

        try{
            $gameRound = $this->gameService->updateGameRounds($game, $request->winner_id, $request->loser_id);
        }catch (\Exception $e){
            return response()->json([], 422);
        }

        event(new GameElementVoted($game, $gameRound->winner));
        event(new GameElementVoted($game, $gameRound->loser));

        // update rank when game complete
        if ($this->gameService->isGameComplete($game)) {
            $anonymousId = session()->get('anonymous_id', 'unknown');
            $game->update(['completed_at' => now()]);
            $candidates = $game->candidates;
            event(new GameComplete($request->user(), $anonymousId, $gameRound, $candidates));
        }

        // retrieve next round
        $elements = $this->getNextElements($game);

        return response()->json([
            'status' => $this->getStatus($game),
            'data' => $elements
        ]);

    }

    protected function getNextElements(Game $game)
    {
        $elements = $this->gameService->takeGameElements($game, 2);
        if($elements->count() > 0){
            $this->gameService->setCandidates($game, $elements->pluck('id')->implode(','));
        }
        return GameRoundResource::make($game, $elements);
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
