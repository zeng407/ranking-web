<?php

namespace App\Http\Controllers\Api;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Events\RefreshGameCandidates;
use App\Helper\AccessTokenService;
use App\Helper\CacheService;
use App\Helper\ClientRequestResolver;
use App\Http\Controllers\Controller;
use App\Http\Resources\Game\GameRoomResource;
use App\Http\Resources\Game\GameRoomUserResource;
use App\Http\Resources\Game\GameRoomVoteResource;
use App\Http\Resources\Game\GameRoundResource;
use App\Http\Resources\Game\HostGameRoomResource;
use App\Http\Resources\PostResource;
use App\Jobs\NotifyGameBet;
use App\Models\Game;
use App\Models\GameRoom;
use App\Models\Post;
use App\Rules\GameCandicateRule;
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;

class GameController extends Controller
{
    use ClientRequestResolver;

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

        return PostResource::make($post);
    }

    public function nextRound(Game $game)
    {
        /** @see \App\Policies\GamePolicy::play() */
        $this->authorize('play', $game);

        $data = $this->getNextElements($game);

        event(new RefreshGameCandidates($game));

        return $data;
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

        $game = $this->gameService->createGame(
            $post,
            $request->element_count,
            $this->getClientIp($request),
            $this->getClientIpContry($request)
        );

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

        // retrieve next round
        $elements = $this->getNextElements($game);

        event(new GameElementVoted($game, $gameRound));

        // update rank when game complete
        if ($this->gameService->isGameComplete($game)) {
            $anonymousId = session()->get('anonymous_id', 'unknown');
            $game->update(['completed_at' => now()]);
            $candidates = $game->candidates;
            event(new GameComplete($request->user(), $anonymousId, $gameRound, $candidates));
        }

        return response()->json([
            'status' => $this->getStatus($game),
            'data' => $elements
        ]);

    }

    public function getRoom(Request $request)
    {
        $gameRoomSerial = $request->route('gameRoom');
        $room = GameRoom::where('serial', $gameRoomSerial)->firstOrFail();

        return GameRoomResource::make($room);
    }

    public function getRoomVotes(Request $request)
    {
        $gameRoomSerial = $request->route('gameRoom');
        $gameSerial = $request->input('game_serial');
        $room = GameRoom::where('serial', $gameRoomSerial)->firstOrFail();
        if($gameSerial != $room->game->serial){
            return response()->json([], 403);
        }

        return GameRoomVoteResource::make($room);
    }

    public function getRoomUser(Request $request)
    {
        $gameRoomSerial = $request->route('gameRoom');
        $room = GameRoom::where('serial', $gameRoomSerial)->firstOrFail();
        $roomUser = $this->gameService->getGameRoomUser($room, $request);

        return GameRoomUserResource::make($roomUser);
    }

    public function bet(Request $request)
    {
        $data = $request->validate([
            'winner_id' => ['required', 'integer'],
            'loser_id' => ['required', 'integer'],
            'current_round' => ['required', 'integer'],
            'of_round' => ['required', 'integer'],
            'remain_elements' => ['required', 'integer'],
        ]);
        $gameRoomSerial = $request->route('gameRoom');

        // todo: use cache for better performance
        $gameRoom = GameRoom::where('serial', $gameRoomSerial)->firstOrFail();
        $gameRoomUser = $this->gameService->getGameRoomUser($gameRoom, $request);
        $this->gameService->bet($gameRoom, $gameRoomUser, $data);

        NotifyGameBet::dispatch($gameRoom);
        return response()->json();
    }

    public function updateGameUser(Request $request)
    {
        $request->validate([
            'nickname' => ['required', 'string', 'max:10'],
        ]);
        $gameRoomSerial = $request->route('gameRoom');
        $gameRoom = GameRoom::where('serial', $gameRoomSerial)->firstOrFail();
        $gameUser = $this->gameService->getGameRoomUser($gameRoom, $request);
        if(CacheService::hasUpdateGameUserNameThreashold($gameUser)){
            return response()->json([], 429);
        }
        $updated = $this->gameService->updateGameRoomUser($gameUser, $request);
        if($updated){
            CacheService::putUpdateGameUserNameThreashold($gameUser);
        }


        return response()->json();
    }

    public function createRoom(Request $request)
    {
        $request->validate([
            'game_serial' => 'required',
        ]);
        $game = $this->getGame($request->input('game_serial'));
        $room = $this->gameService->createGameRoom($game);
        return HostGameRoomResource::make($room);
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
