<?php


namespace App\Services;


use App\Helper\CacheService;
use App\Helper\Locker;
use App\Helper\SerialGenerator;
use App\Http\Resources\Game\GameResultResource;
use App\Jobs\AttachGameElements;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\GameRoom;
use App\Models\GameRoomUser;
use App\Models\GameRoomUserBet;
use App\Models\Post;
use App\Models\User;
use App\Models\UserGameResult;
use Illuminate\Http\Request;
use Illuminate\Validation\ValidationException;
use Ramsey\Uuid\Uuid;

class GameService
{
    public function takeGameElements(Game $game, $count = 2)
    {
        $elements = $game->elements()
            ->wherePivot('is_eliminated', false)
            ->orderByPivot('is_ready', 'desc')
            ->orderByPivot('win_count')
            ->inRandomOrder()
            ->take($count)
            ->get();

        return $elements;
    }

    public function setCandidates(Game $game, string $candidates)
    {
        $game->update([
            'candidates' => $candidates
        ]);
    }

    public function createGame(Post $post,int $elementCount, ?int $userId = null, ?string $ip = null, ?string $ipCountry = null): Game
    {
        /** @var Game $game */
        $game = $post->games()->create([
            'user_id' => $userId,
            'serial' => Uuid::uuid1()->toString(),
            'element_count' => $elementCount,
            'ip' => $ip,
            'ip_country' => $ipCountry
        ]);

        // pick random elements
        $elements = $post->elements()
            ->inRandomOrder()
            ->take($game->element_count)
            ->get(['elements.id']);

        // attch first 32 elements
        $firstElements = $elements->take(32);
        $firstElements->each(function (Element $element) use ($game) {
            $game->elements()->attach($element, [
                'is_ready' => true
            ]);
        });

        // attach rest elements
        $restElements = $elements->diff($firstElements);
        AttachGameElements::dispatch($restElements, $game)->delay(now()->addSeconds(5));

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

    public function getWinner(Game $game)
    {
        $winner = optional($game->game_1v1_rounds()
            ->where('remain_elements', 1)
            ->first())->winner;

        // append data
        if($winner){
            $winner->imgur_url = $winner->imgur_image?->link;
        }

        return $winner;
    }

    public function getGameResult(Request $request, Game $game)
    {
        /** @var RankService */
        $rankService = app(RankService::class);
        $rounds = $game->game_1v1_rounds()
            ->orderBy('remain_elements')
            ->take(9)
            ->get();

        $winner = $this->getWinner($game);
        $rankReport = $rankService->getRankReportByElement($game->post, $winner);

        $gameResult = GameResultResource::collection($rounds)
            ->additional([
                'game_serial' => $game->serial,
                'winner' => $winner,
                'winner_rank' => $rankReport?->rank,
                'winner_win_rate' => $rankReport?->win_rate,
                'statistics' => [
                    'timeline' => $this->getGameTimeline($game),
                    'game_time' => $game->created_at->diffInSeconds($game->completed_at),
                    'winner_id' => $winner->id,
                    'winner_global_rank' => $rankReport?->rank
                ],
                'rounds' => $game->element_count,
                'game_room' => $game->game_room ? CacheService::rememberGameBetRank($game->game_room, true) : null,
            ]);

        return $gameResult;
    }

    public function updateGameRounds(Game $game, $winnerId, $loserId): Game1V1Round
    {
        $lastRound = $game->game_1v1_rounds()->latest('id')->first();
        if ($lastRound === null) {
            $round = 1;
            $ofRound = (int) ceil($game->element_count / 2);
            $remain = $game->element_count - 1;
        } else if ($lastRound->current_round + 1 > $lastRound->of_round) {
            $round = 1;
            $ofRound = $this->calculateNextRoundNumber($lastRound->remain_elements);
            $remain = $lastRound->remain_elements - 1;
        } else {
            $round = $lastRound->current_round + 1;
            $ofRound = $lastRound->of_round;
            $remain = $lastRound->remain_elements - 1;
        }
        $data = [
            'post_id' => $game->post_id,
            'current_round' => $round,
            'of_round' => $ofRound,
            'remain_elements' => $remain,
            'winner_id' => $winnerId,
            'loser_id' => $loserId
        ];
        logger('saving game : ' . $game->id, $data);

        try {
            $lock = Locker::lockUpdateGameElement($game);
            $lock->block(5);
            $isEndOfRound = $round === $ofRound;

            \DB::transaction(function () use ($game, $winnerId, $loserId, $isEndOfRound) {
                // update winner
                $gameElement = $game->game_elements()
                    ->where('element_id', $winnerId)
                    ->where('is_eliminated', false)
                    ->first();
                if($gameElement){
                    $gameElement->update([
                        'win_count' => $gameElement->win_count + 1,
                        'is_ready' => false
                    ]);
                    \Log::debug('game element updated', ['game_id' => $game->id, 'element_id' => $winnerId, 'win_count' => $gameElement->win_count]);
                }else{
                    \Log::error('game element not found', ['game_id' => $game->id, 'element_id' => $winnerId]);
                    throw new \Exception('game element not found');
                }

                // update loser
                $gameElement = $game->game_elements()
                    ->where('element_id', $loserId)
                    ->where('is_eliminated', false)
                    ->first();
                if($gameElement){
                    $gameElement->update([
                        'is_eliminated' => true,
                        'is_ready' => false
                    ]);
                    \Log::debug('game element updated', ['game_id' => $game->id, 'element_id' => $loserId, 'is_eliminated' => $gameElement->is_eliminated]);
                }else{
                    \Log::error('game element not found', ['game_id' => $game->id, 'element_id' => $loserId]);
                    throw new \Exception('game element not found');
                }

                if($isEndOfRound){
                    $game->game_elements()
                        ->where('is_eliminated', false)
                        ->update([
                            'is_ready' => true
                        ]);
                }
                $game->update([
                    'vote_count' => $game->vote_count + 1,
                    'user_id' => request()->user()?->id
                ]);
            });

            logger('lock release', ['game_id' => $game->id]);
            $lock->release();
        } catch (\Exception $e) {
            \Log::error('game update failed', ['game_id' => $game->id, 'winner_id' => $winnerId, 'loser_id' => $loserId]);
            report($e);
            $lock->release();
            throw $e;
        }
        return $game->game_1v1_rounds()->create($data);
    }

    /**
     * 核心演算法：根據現在是第幾輪，計算這一輪該打幾場 (淘汰幾人)
     * 規則：
     * R1: 強制淘汰一半 (68 -> 34)
     * R2: 修正為 2^n (34 -> 32, 需淘汰 2 人)
     * R3+: 淘汰一半 (32 -> 16)
     */
    private function calculateMatchesForRound(int $round, int $remainElements): int
    {
        // 第一輪：永遠淘汰一半
        if ($round === 1) {
            return (int) floor($remainElements / 2);
        }

        // 第二輪：檢查是否為 2^n，如果是多出來的就要淘汰
        if ($round === 2) {
            // 找出最接近且小於等於 remain 的 2 的次方數 (例如 34 -> 32)
            $powerOf2 = 1;
            while (($powerOf2 * 2) <= $remainElements) {
                $powerOf2 *= 2;
            }

            $diff = $remainElements - $powerOf2;

            // 如果有零頭 (例如 34-32=2)，這一輪就只打這幾場修正賽
            if ($diff > 0) {
                return $diff;
            }

            // 如果剛好是 2^n (例如 32)，那就正常淘汰一半
            return (int) floor($remainElements / 2);
        }

        // 第三輪以後：都是淘汰一半
        return (int) floor($remainElements / 2);
    }

    /**
     * 獲取遊戲參賽元素清單
     *
     * @param Game $game
     * @param int $limit
     * @return \Illuminate\Database\Eloquent\Collection
     */
    public function getGameElements(Game $game, int $limit)
    {
        return $game->elements()
            // 預加載圖片關聯，避免 N+1 (參考 getWinner 的用法)
            ->with('imgur_image')
            // 撈取 Pivot 表上的遊戲狀態
            ->withPivot(['win_count', 'is_eliminated', 'is_ready'])
            // 排序邏輯：未淘汰的優先，然後按勝場數排序 (可依需求調整)
            ->orderByPivot('is_eliminated', 'asc') // 活著的在前面
            ->orderByPivot('win_count', 'asc')
            ->take($limit)
            ->get();
    }

    /**
     * 驗證批次投票 (更新版邏輯)
     */
    private function validateBatchVotes(Game $game, array $votes)
    {
        // ... (前面的 ID 檢查、重複淘汰檢查 保持不變) ...
        $winnerIds = collect($votes)->pluck('winner_id');
        $loserIds = collect($votes)->pluck('loser_id');
        $allIds = $winnerIds->merge($loserIds)->unique();

        // 1. 基本檢查
        $count = $game->elements()->whereIn('elements.id', $allIds)->count();
        if ($count !== $allIds->count()) {
            throw ValidationException::withMessages(['votes' => 'Invalid elements.']);
        }

        $alreadyEliminated = $game->elements()
            ->wherePivot('is_eliminated', true)
            ->pluck('elements.id')->toArray();
        $batchEliminated = [];

        // 2. 模擬賽制狀態
        // 從 DB 取得「目前的狀態」作為起點
        $lastRound = $game->game_1v1_rounds()->latest('id')->first();

        if ($lastRound) {
            $simRound = $lastRound->current_round;
            $simRemain = $lastRound->remain_elements;

            // 這裡很關鍵：要算一下這一輪「已經」打了幾場，加上這次 batch 的第一張票會不會溢出
            // 取得 DB 中這一輪已經產生的場次數量 (例如 R1 應打 34 場，DB 已有 30 場)
            $matchesPlayedInCurrentRound = $game->game_1v1_rounds()
                ->where('current_round', $simRound)
                ->count();

            // 計算這一輪「總共」該有幾場 (使用新邏輯)
            $matchesNeededForCurrentRound = $lastRound->of_round;
        } else {
            // 遊戲剛開始
            $simRound = 1;
            $simRemain = $game->element_count;
            $matchesPlayedInCurrentRound = 0;
            // 計算 R1 該打幾場 (68 -> 34)
            $matchesNeededForCurrentRound = $this->calculateMatchesForRound(1, $simRemain);
        }

        ksort($votes); // 確保按順序模擬

        foreach ($votes as $index => $vote) {
            $winnerId = $vote['winner_id'];
            $loserId = $vote['loser_id'];

            if (in_array($winnerId, $alreadyEliminated) || in_array($winnerId, $batchEliminated)) {
                throw ValidationException::withMessages(["votes.{$index}" => "Winner {$winnerId} eliminated."]);
            }
            if (in_array($loserId, $alreadyEliminated) || in_array($loserId, $batchEliminated)) {
                throw ValidationException::withMessages(["votes.{$index}" => "Loser {$loserId} eliminated."]);
            }
            $batchEliminated[] = $loserId;

            // --- 模擬推進邏輯 ---
            $matchesPlayedInCurrentRound++;

            // 檢查是否超過當前輪次上限
            if ($matchesPlayedInCurrentRound > $matchesNeededForCurrentRound) {
                // 進入下一輪
                $simRound++;

                // 更新剩餘人數 (上一輪打了 matchesNeeded 場，所以淘汰了這麼多人)
                // 例如 68人，R1 打了 34 場，剩 34 人
                // 注意：這裡是用上一輪的「總目標」來扣，而不是用 batch 跑的次數
                $simRemain = $simRemain - $matchesNeededForCurrentRound;

                // 重置計數器 (這一票是新的一輪的第一場)
                $matchesPlayedInCurrentRound = 1;

                // 計算新的一輪需要打幾場 (使用新邏輯: 34 -> 2)
                $matchesNeededForCurrentRound = $this->calculateMatchesForRound($simRound, $simRemain);
            }
        }
    }

    /**
     * 計算特定階段 (Stage) 應有的總場次 (of_round)
     * Stage 1: 強制淘汰一半 (ceil)
     * Stage 2: 修正至 2^n
     * Stage 3+: 標準淘汰一半 (floor)
     */
    private function calculateMatchesForStage(int $stage, int $remainElements): int
    {
        // Stage 1: 強制淘汰一半 (例如 45人 -> 23場, 300人 -> 150場)
        if ($stage === 1) {
            return (int) ceil($remainElements / 2);
        }

        // Stage 2: 修正輪 (修正至最接近的 2^n)
        if ($stage === 2) {
            // 找出最接近且小於等於 remain 的 2 的次方 (例如 150 -> 128)
            $powerOf2 = 1;
            while (($powerOf2 * 2) <= $remainElements) {
                $powerOf2 *= 2;
            }

            $diff = $remainElements - $powerOf2;

            // 如果有零頭 (例如 150-128=22)，這一輪就只打 22 場
            if ($diff > 0) {
                return $diff;
            }

            // 如果剛好是 2^n，那就正常淘汰一半
            return (int) floor($remainElements / 2);
        }

        // Stage 3+: 標準淘汰 (例如 128 -> 64場)
        return (int) floor($remainElements / 2);
    }

    /**
     * @return Game1V1Round|null
     */
    public function batchUpdateGameRounds(Game $game, array $votes)
    {
        $this->validateBatchVotes($game, $votes);

        ksort($votes);
        $lock = Locker::lockUpdateGameElement($game);
        $lock->block(10);

        try {
            $lastCreatedRound = null;
            $lastRound = $game->game_1v1_rounds()->latest('id')->first();

            // 1. 判斷目前的 Stage (第幾輪)
            // 透過計算資料庫中有多少次 "current_round = 1" 來得知目前是第幾階段
            $stageCount = $game->game_1v1_rounds()->where('current_round', 1)->count();

            // 2. 初始化狀態變數
            if ($lastRound === null) {
                // 遊戲剛開始
                $stage = 1;
                $remain = $game->element_count;
                $matchIndex = 0; // 下一場是 1
                $matchesInStage = $this->calculateMatchesForStage($stage, $remain);
            } else {
                $remain = $lastRound->remain_elements;

                // 判斷上一筆紀錄是否為該輪的最後一場
                if ($lastRound->current_round >= $lastRound->of_round) {
                    // 上一輪已結束，準備進入下一輪
                    $stage = $stageCount + 1;
                    $matchIndex = 0;
                    $matchesInStage = $this->calculateMatchesForStage($stage, $remain);
                } else {
                    // 還在同一輪
                    $stage = $stageCount > 0 ? $stageCount : 1; // 防呆
                    $matchIndex = $lastRound->current_round;
                    $matchesInStage = $lastRound->of_round;
                }
            }

            \DB::transaction(function () use ($game, $votes, &$stage, &$matchIndex, &$matchesInStage, &$remain, &$lastCreatedRound) {
                foreach ($votes as $vote) {
                    $winnerId = $vote['winner_id'];
                    $loserId = $vote['loser_id'];

                    // 每一票代表一人淘汰，剩餘人數 -1
                    $remain--;

                    // 場次 +1
                    $matchIndex++;

                    if ($matchIndex > $matchesInStage) {
                        // 進入下一輪
                        $stage++;
                        $matchIndex = 1; // 重置為第 1 場

                        // 重新計算 matchesInStage
                        $matchesInStage = $this->calculateMatchesForStage($stage, $remain + 1);
                    }

                    $isEndOfRound = ($matchIndex === $matchesInStage);

                    // --- DB Updates ---
                    \DB::table('game_elements')
                        ->where('game_id', $game->id)->where('element_id', $winnerId)
                        ->update(['win_count' => \DB::raw('win_count + 1'), 'is_ready' => false]);

                    \DB::table('game_elements')
                        ->where('game_id', $game->id)->where('element_id', $loserId)
                        ->update(['is_eliminated' => true, 'is_ready' => false]);

                    if ($isEndOfRound) {
                        \DB::table('game_elements')
                            ->where('game_id', $game->id)->where('is_eliminated', false)
                            ->update(['is_ready' => true]);
                    }

                    // 建立紀錄
                    // current_round: 目前是第幾場
                    // of_round: 這一輪總共幾場
                    $lastCreatedRound = $game->game_1v1_rounds()->create([
                        'post_id' => $game->post_id,
                        'current_round' => $matchIndex,
                        'of_round' => $matchesInStage,
                        'remain_elements' => $remain,
                        'winner_id' => $winnerId,
                        'loser_id' => $loserId
                    ]);

                    $game->increment('vote_count');
                }
            });

            $lock->release();
            return $lastCreatedRound;

        } catch (\Exception $e) {
            $lock->release();
            throw $e;
        }
    }

    public function calculateNextRoundNumber($remain)
    {
        $ofRound = $remain;
        for ($i = config('setting.post_max_element_count'); $i >= 1; $i /= 2) {
            if ($remain > $i) {
                $ofRound = $remain - $i;
                break;
            }
        }
        return $ofRound;
    }

    public function createUserGameResult(?User $user, string $anonymousId, Game1V1Round $game1V1Round, string $candidates): UserGameResult
    {
        $data = [
            'user_id' => $user?->id,
            'anonymous_id' => $anonymousId,
            'game_id' => $game1V1Round->game_id,
            'champion_id' => $game1V1Round->winner_id,
            'champion_name' => $game1V1Round->winner->title ?? '',
            'loser_id' => $game1V1Round->loser_id,
            'loser_name' => $game1V1Round->loser->title ?? '',
            'candidates' => $candidates
        ];
        return UserGameResult::create($data);
    }

    public function getGameTimeline(Game $game)
    {
        $rounds = $game->game_1v1_rounds()
            ->orderBy('id')
            ->get();
        // push game to first
        $rounds = collect([$game])->concat($rounds);

        // get diff in every 2 timestamps
        $timeline = $rounds->map(function ($round, $key) use ($rounds, $game) {
            if ($key === 0) {
                return null;
            }
            return [
                'diff' => $round->created_at->diffInSeconds($rounds[$key - 1]->created_at),
                'winner' => $round->winner_id,
                'winner_name' => $round->winner->title,
                'loser' => $round->loser_id,
                'loser_name' => $round->loser->title,
                'current_round' => $round->current_round,
                'of_round' => $round->of_round,
                'start_at' => $rounds[$key - 1]->created_at->format('Y-m-d H:i:s'),
                'end_at' => $round->created_at->format('Y-m-d H:i:s'),
                'rounds' => $game->element_count - $round->remain_elements,
            ];
        });
        $timeline->shift();
        return $timeline;
    }

    public function getGameRoomUser(GameRoom $gameRoom, Request $request)
    {
        $user = $request->user();
        $anonymousId = $request->session()->get('anonymous_id', 'unknown');
        $gameRoomUser = $gameRoom->users()
            ->where(function($query)use($user, $anonymousId){
                if($user){
                    $query->where('user_id', $user->id)
                        ->orWhere('anonymous_id', $anonymousId);
                }else{
                    $query->where('anonymous_id', $anonymousId);
                }
            })
            ->first();

        if(!$gameRoomUser){
            $gameRoomUser = $gameRoom->users()->create([
                'user_id' => $user?->id,
                'anonymous_id' => $anonymousId,
                'score' => config('setting.default_bet_score'),
                'nickname' => random_nickname(),
                'rank' => 0,
                'accuracy' => 0,
                'total_played' => 0,
                'total_correct' => 0,
            ]);
        }else{
            $gameRoomUser->update([
                'user_id' => $user?->id
            ]);
        }
        return $gameRoomUser;
    }

    public function updateGameRoomUser(GameRoomUser $gameRoomUser, Request $request)
    {
        return $gameRoomUser->update([
            'nickname' => $request->input('nickname')
        ]);
    }

    public function bet(GameRoom $gameRoom, GameRoomUser $gameRoomUser, array $data)
    {
        $lastRound = $gameRoomUser->bets()
            ->select(['last_combo', 'won_at'])
            ->orderByDesc('id')
            ->first();
        $lastRoundCombo = $lastRound ? $lastRound->last_combo : 0;
        $isWon = $lastRound ? $lastRound->won_at !== null : false;
        $combo = $isWon ? ($lastRoundCombo + 1) : 0;

        $gameRoomUser->bets()->updateOrCreate([
            'game_room_id' => $gameRoom->id,
            'game_room_user_id' => $gameRoomUser->id,
            'current_round' => $data['current_round'],
            'of_round' => $data['of_round'],
            'remain_elements' => $data['remain_elements'],
        ],[
            'game_room_id' => $gameRoom->id,
            'game_room_user_id' => $gameRoomUser->id,
            'current_round' => $data['current_round'],
            'of_round' => $data['of_round'],
            'remain_elements' => $data['remain_elements'],
            'winner_id' => $data['winner_id'],
            'loser_id' => $data['loser_id'],
            'last_combo' => $combo,
        ]);
    }

    public function getChannelConnectionCount(GameRoom $gameRoom)
    {
        $channel = "game-room.{$gameRoom->serial}";
        return CacheService::rememebrChannelSubscriptionCount($channel);
    }

    public function getCurrentElements(Game $game)
    {
        if($game->completed_at){
            $elements = [];
        }else{
            $elementsId = explode(',', $game->candidates);
            $unsortElements = $game->elements()
                ->whereIn('elements.id', $elementsId)
                ->get();
            $elements = [
                $unsortElements->where('id', $elementsId[0])->first(),
                $unsortElements->where('id', $elementsId[1])->first(),
            ];
        }

        return $elements;
    }

    public function createGameRoom(Game $game) : \App\Models\GameRoom
    {
        return $game->game_room()->firstOrCreate([], [
            'serial' => SerialGenerator::genGameRoomSerial()
        ]);
    }

    public function updateGameBet(GameRoom $gameRoom, $winnerId, $loserId, array $conditions)
    {
        logger('updateGameBet', $conditions);
        $comboScore = config('setting.bet_combo_score');
        $wonScore = config('setting.bet_won_score');
        $loseScore = config('setting.bet_lose_score');
        GameRoomUserBet::where('game_room_id', $gameRoom->id)
            ->where('current_round', $conditions['current_round'])
            ->where('of_round', $conditions['of_round'])
            ->where('remain_elements', $conditions['remain_elements'] + 1)
            ->where('winner_id', $winnerId)
            ->where('loser_id', $loserId)
            ->update([
                'won_at' => now(),
                'score' => \DB::raw("last_combo * {$comboScore} + {$wonScore}")
            ]);

        GameRoomUserBet::where('game_room_id', $gameRoom->id)
            ->where('current_round', $conditions['current_round'])
            ->where('of_round', $conditions['of_round'])
            ->where('remain_elements', $conditions['remain_elements'] + 1)
            ->where('winner_id', $loserId)
            ->where('loser_id', $winnerId)
            ->update([
                'lost_at' => now(),
                'score' => $loseScore
            ]);

        // remove won_at and lost_at are null
        GameRoomUserBet::where('game_room_id', $gameRoom->id)
            ->where('current_round', $conditions['current_round'])
            ->where('of_round', $conditions['of_round'])
            ->where('remain_elements', $conditions['remain_elements'] + 1)
            ->whereNull('won_at')
            ->whereNull('lost_at')
            ->delete();
    }

    public function updateGameRoomUserBetScore(GameRoomUser $gameRoomUser)
    {
        $currentPlayed = $gameRoomUser->total_played;

        $bets = $gameRoomUser->bets()
            ->select(['last_combo','score', 'won_at'])
            ->orderBy('id')
            ->limit($currentPlayed + 1)
            ->get();

        $totalPlayed = 0;
        $totalCorrect = 0;
        $score = config('setting.default_bet_score');
        foreach ($bets as $bet){
            $totalPlayed++;
            if($bet->won_at){
                $totalCorrect++;
            }
            $score += $bet->score;
        }
        $accuracy = $totalPlayed > 0 ? $totalCorrect / $totalPlayed * 100 : 0;

        $lastBet = $bets->last();
        if($lastBet && $lastBet->won_at){
            $combo = $lastBet->last_combo + 1;
        }else{
            $combo = 0;
        }

        $gameRoomUser->update([
            'combo' => $combo,
            'score' => $score,
            'accuracy' => $accuracy,
            'total_played' => $totalPlayed,
            'total_correct' => $totalCorrect
        ]);
    }

    public function getUserLastGameSerial(Post $post, ?User $user)
    {
        if(!$user){
            return null;
        }

        $gmae = Game::where('post_id', $post->id)
            ->where('user_id', $user?->id)
            ->orderByDesc('id')
            ->select(['completed_at', 'serial'])
            ->first();

        if($gmae && !$gmae->completed_at){
            return $gmae->serial;
        }

        return null;
    }
}
