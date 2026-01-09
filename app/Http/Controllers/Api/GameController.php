<?php

namespace App\Http\Controllers\Api;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Events\RefreshGameCandidates;
use App\Helper\AccessTokenService;
use App\Helper\CacheService;
use App\Helper\ClientRequestResolver;
use App\Http\Controllers\Controller;
use App\Http\Resources\Game\GameElementResource;
use App\Http\Resources\Game\GameRoomResource;
use App\Http\Resources\Game\GameRoomUserResource;
use App\Http\Resources\Game\GameRoomVoteResource;
use App\Http\Resources\Game\GameRoundResource;
use App\Http\Resources\Game\HostGameRoomResource;
use App\Http\Resources\PostResource;
use App\Jobs\NotifyGameBet;
use App\Jobs\UpdateBatchElementRanks;
use App\Models\Game;
use App\Models\GameRoom;
use App\Models\Post;
use App\Rules\GameCandicateRule;
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

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

    /**
     * 取得遊戲的所有參賽元素 (支援指定數量)
     *
     * @param Request $request
     * @param Game $game
     * @return \Illuminate\Http\JsonResponse
     */
    public function getGameElements(Request $request, Game $game)
    {
        $request->validate([
            // 限制最大只能撈取 1024 筆
            'limit' => 'nullable|integer|min:1|max:1024'
        ]);

        // 檢查讀取權限 (參考 getSetting 的邏輯)
        // 確保如果 Post 是設密碼的，需要驗證 Authorization
        /** @see \App\Policies\PostPolicy::publicRead() */
        $this->authorize('public-read', [$game->post, $request->header('Authorization')]);

        // 預設最大 1024，或使用使用者指定的 limit
        $limit = $request->input('limit', 1024);

        $elements = $this->gameService->getGameElements($game, $limit);
        $elements = GameElementResource::collection($elements);
        return response()->json([
            'game_serial' => $game->serial,
            'total_count' => $game->element_count, // 回傳該局總人數
            'listed_count' => $elements->count(),   // 回傳本次列出人數
            'data' => $elements
        ]);
    }

    public function roomRank(Request $request, Game $game)
    {
        if($game->game_room === null){
            return response()->json([
                'ranks' => [],
                'rank_updating' => false
            ], 200);
        }
        return [
            'ranks' => CacheService::rememberGameBetRank($game->game_room),
            'rank_updating' => CacheService::hasUpdatingGameRoomRank($game->game_room),
        ];
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
            $request->user()?->id,
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
        $start = microtime(true);
        $lastMark = $start;
        $timings = [];
        $mark = function (string $label) use (&$timings, &$lastMark) {
            $now = microtime(true);
            $timings[$label] = round(($now - $lastMark) * 1000, 2);
            $lastMark = $now;
        };

        $request->validate([
            'game_serial' => 'required',
        ]);
        $mark('validate_serial');
        /** @var Game $game */
        $game = $this->getGame($request->input('game_serial'));
        $mark('load_game');
        logger("game candidate before vote", ['candidates' => $game->candidates]);
        $request->validate([
            'winner_id' => ['required', 'different:loser_id', new GameCandicateRule($game)],
            'loser_id' => ['required', 'different:winner_id', new GameCandicateRule($game)]
        ]);
        $mark('validate_candidates');

        /** @see \App\Policies\GamePolicy::play() */
        $this->authorize('play', $game);
        $mark('authorize');

        try{
            $gameRound = $this->gameService->updateGameRounds($game, $request->winner_id, $request->loser_id);
            $mark('update_round');
        }catch (\Exception $e){
            return response()->json([], 422);
        }

        // retrieve next round
        $elements = $this->getNextElements($game);
        $mark('next_elements');

        // Add to processing job
        if($game->game_room){
            CacheService::putUpdatingGameRoomRank($game->game_room);
        }
        $mark('flag_room_rank');
        event(new GameElementVoted($game, $gameRound));
        $mark('event_voted');

        $isComplete = ((int) ($gameRound->remain_elements ?? 0)) === 1;
        $mark('is_game_complete');

        // update rank when game complete
        if ($isComplete) {
            $mark('check_complete');
            $anonymousId = session()->get('anonymous_id', 'unknown');
            $game->update(['completed_at' => now()]);
            $candidates = $game->candidates;
            event(new GameComplete($request->user(), $anonymousId, $gameRound, $candidates));
            $mark('complete_flow');
        }

        $status = $isComplete ? self::END_GAME : self::PROCESSING;
        $mark('status');

        $totalMs = round((microtime(true) - $start) * 1000, 2);
        $sumMs = round(array_sum($timings), 2);
        $gapMs = round($totalMs - $sumMs, 2);

        logger('vote timings', [
            'game_id' => $game->id,
            'timings_ms' => $timings,
            'sum_ms' => $sumMs,
            'gap_ms' => $gapMs,
            'total_ms' => $totalMs,
        ]);

        return response()->json([
            'status' => $status,
            'data' => $elements
        ]);

    }

    /**
     * 處理批次投票
     * * @param Request $request
     * @return \Illuminate\Http\JsonResponse
     */
    public function batchVote(Request $request)
    {
        $request->validate([
            'game_serial' => 'required',
            // 驗證 votes 是一個陣列，且 key 為回合數 (雖然 Service 會重新計算順序，但驗證結構很重要)
            'votes' => 'array|max:1023',
            'votes.*.winner_id' => 'required|integer|different:votes.*.loser_id',
            'votes.*.loser_id' => 'required|integer',
            'current_candidates' => 'nullable|array|size:2'
        ]);

        /** @var Game $game */
        $game = $this->getGame($request->input('game_serial'));

        /** @see \App\Policies\GamePolicy::play() */
        $this->authorize('play', $game);

        $lastGameRound = null;

        try {
            // 先去除重複的 winner/loser 組合，避免前端重試造成重複票
            $votes = $request->input('votes', []);
            $uniqueVotes = [];
            $seenPairs = [];

            // 查 DB 已存在的回合，跳過已處理的組合
            $existingPairs = DB::table('game_1v1_rounds')
                ->where('game_id', $game->id)
                ->select('winner_id', 'loser_id')
                ->get()
                ->map(function ($row) {
                    return $row->winner_id . '-' . $row->loser_id;
                })
                ->flip();

            foreach ($votes as $vote) {
                $pairKey = $vote['winner_id'] . '-' . $vote['loser_id'];

                // 已處理過 (DB 或本批次) 就跳過
                if (isset($existingPairs[$pairKey]) || isset($seenPairs[$pairKey])) {
                    continue;
                }

                $seenPairs[$pairKey] = true;
                $uniqueVotes[] = $vote;
            }

            // 呼叫 Service 處理批次更新
            $lastGameRound = $this->gameService->batchUpdateGameRounds($game, $uniqueVotes);

            //收集所有變動過的 Element IDs
            if (!empty($uniqueVotes)) {
                $elementIds = collect($uniqueVotes)
                    ->flatMap(function ($vote) {
                        return [$vote['winner_id'], $vote['loser_id']];
                    })
                    ->unique()
                    ->values()
                    ->toArray();

                //發送 Queue Job 進行排名計算
                if (!empty($elementIds)) {
                    UpdateBatchElementRanks::dispatch($game->post, $elementIds);
                }
            }

        } catch (\Exception $e) {
            \Log::error('Batch vote failed', ['error' => $e->getMessage(), 'trace' => $e->getTraceAsString()]);
            return response()->json(['message' => 'Batch processing failed'], 422);
        }

        // 檢查遊戲是否結束
        if ($this->gameService->isGameComplete($game)) {
            // 如果前面沒有回傳最後一回合，補抓最新一筆避免 null 取屬性
            if (!$lastGameRound) {
                \Log::warning('Game complete but lastGameRound missing, fetching last round', ['game_id' => $game->id]);
                $lastGameRound = DB::table('game_1v1_rounds')
                    ->where('game_id', $game->id)
                    ->orderByDesc('id')
                    ->first();
            }

            if ($lastGameRound) {
                $anonymousId = session()->get('anonymous_id', 'unknown');
                $game->update(['completed_at' => now()]);
                $candidates = "{$lastGameRound->winner_id},{$lastGameRound->loser_id}";
                // 觸發遊戲完成事件
                event(new GameComplete($request->user(), $anonymousId, $lastGameRound, $candidates));
            } else {
                \Log::warning('Game complete but no lastGameRound found', ['game_id' => $game->id]);
            }
        }

        $elements = $this->gameService->getCurrentElements($game);
        return response()->json([
            'status' => $this->getStatus($game),
            'data' => GameRoundResource::make($game, $elements)
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
            'current_candidates' => 'nullable|array|size:2',
            'current_candidates.*' => 'integer'
        ]);
        $game = $this->getGame($request->input('game_serial'));
        $room = $this->gameService->createGameRoom($game);

        if ($request->has('current_candidates')
            && $game->elements()->whereIn('element_id', $request->current_candidates)->count() == 2) {
            $candidates = $request->current_candidates;
            $leftId = $candidates[0];
            $rightId = $candidates[1];
            logger("set candidates on room creation", ['candidates' => $candidates]);
            $this->gameService->setCandidates($game, "{$leftId},{$rightId}");
        }

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
