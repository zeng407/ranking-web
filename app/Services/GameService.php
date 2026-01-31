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
use Exception;
use Illuminate\Database\QueryException;
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

        // attach first 128 elements
        $firstElements = $elements->take(128);
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
        if ($game->completed_at !== null) {
            return true;
        }

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

    public function getGameResult(Game $game)
    {
        return CacheService::rememberGameResult($game, function () use ($game) {
            /** @var RankService */
            $rankService = app(RankService::class);
            $rounds = $game->game_1v1_rounds()
                ->orderBy('remain_elements')
                ->take(9)
                ->get();

            $winner = $this->getWinner($game);
            $rankReport = $rankService->getRankReportByElement($game->post, $winner);

            $resource = GameResultResource::collection($rounds)
                ->additional([
                    'game_serial' => $game->serial,
                    'winner' => $winner,
                    'winner_rank' => $rankReport?->rank,
                    'winner_win_rate' => $rankReport?->win_rate,
                    'statistics' => [
                        'timeline' => $this->getGameTimeline($game),
                        'game_time' => $game->created_at->diffInSeconds($game->completed_at),
                        'winner_id' => $winner?->id,
                        'winner_global_rank' => $rankReport?->rank
                    ],
                    'rounds' => $game->element_count,
                    'game_room' => $game->game_room ? CacheService::rememberGameBetRank($game->game_room, true) : null,
                ]);

            // Convert to array before caching to avoid serialization issues
            return $resource->response()->getData(true);
        });
    }

    public function updateGameRounds(Game $game, $winnerId, $loserId): Game1V1Round
    {
        $lock = Locker::lockUpdateGameElement($game);
        $lock->block(5);

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

        $isEndOfRound = $round === $ofRound;
        $winner = $game->game_elements()
            ->where('element_id', $winnerId)
            ->where('is_eliminated', false)
            ->first();
        $loser = $game->game_elements()
            ->where('element_id', $loserId)
            ->where('is_eliminated', false)
            ->first();

        try {
            \DB::transaction(function () use ($game, $winner, $loser, $isEndOfRound) {
                // update winner
                if($winner){
                    $winner->update([
                        'win_count' => $winner->win_count + 1,
                        'is_ready' => false
                    ]);
                    \Log::debug('game element updated', ['game_id' => $game->id, 'element_id' => $winner->element_id, 'win_count' => $winner->win_count]);
                }else{
                    \Log::error('game element not found', ['game_id' => $game->id]);
                    throw new \Exception('game element not found');
                }

                // update loser
                if($loser){
                    $loser->update([
                        'is_eliminated' => true,
                        'is_ready' => false
                    ]);
                    \Log::debug('game element updated', ['game_id' => $game->id, 'element_id' => $loser->element_id, 'is_eliminated' => $loser->is_eliminated]);
                }else{
                    \Log::error('game element not found', ['game_id' => $game->id]);
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
            ->with('imgur_image')
            ->withPivot(['win_count', 'is_eliminated', 'is_ready'])
            // 排序邏輯：未淘汰的優先，然後按勝場數排序 (可依需求調整)
            ->orderByPivot('is_eliminated', 'asc') // 活著的在前面
            ->orderByPivot('win_count', 'asc')
            ->take($limit)
            ->get();
    }

    /**
     * 驗證批次投票
     */
    private function validateBatchVotes(Game $game, array $votes)
    {
        $winnerIds = collect($votes)->pluck('winner_id');
        $loserIds = collect($votes)->pluck('loser_id');
        $allIds = $winnerIds->merge($loserIds)->unique();

        // 1. 基本檢查
        $count = $game->elements()->whereIn('elements.id', $allIds)->count();
        if ($count !== $allIds->count()) {
            \Log::error("Some elements do not belong to the game.", [
                'game_id' => $game->id,
                'expected_count' => $allIds->count(),
                'found_count' => $count,
                'element_ids' => $allIds->toArray()
            ]);
            throw new Exception("Some elements do not belong to the game {$game->id}. Expected {$allIds->count()}, found {$count}.");
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

            $matchesPlayedInCurrentRound = $lastRound->current_round;

            // 計算這一輪總共該有幾場
            $matchesNeededForCurrentRound = $lastRound->of_round;
        } else {
            // 遊戲剛開始
            $simRound = 1;
            $simRemain = $game->element_count;
            $matchesPlayedInCurrentRound = 0;
            // 計算該打幾場 (68人 -> 34場)
            $matchesNeededForCurrentRound = $this->calculateMatchesForRound(1, $simRemain);
        }

        ksort($votes); // 確保按順序模擬

        foreach ($votes as $index => $vote) {
            $winnerId = $vote['winner_id'];
            $loserId = $vote['loser_id'];

            if (in_array($winnerId, $alreadyEliminated) || in_array($winnerId, $batchEliminated)) {
                \Log::error("Winner already eliminated.", [
                    'votes' => $votes,
                ]);
                throw new Exception("Game {$game->id} Winner {$winnerId} eliminated.");
            }
            if (in_array($loserId, $alreadyEliminated) || in_array($loserId, $batchEliminated)) {
                \Log::error("Loser already eliminated.", [
                    'votes' => $votes,
                ]);
                throw new Exception("Game {$game->id} Loser {$loserId} eliminated.");
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
     * Deadlock-aware update for game_elements rows.
     */
    private function updateGameElementWithRetry(int $gameId, int $elementId, array $data, int $attempts = 3, int $backoffMs = 100): void
    {
        for ($i = 0; $i < $attempts; $i++) {
            try {
                \DB::table('game_elements')
                    ->where('game_id', $gameId)
                    ->where('element_id', $elementId)
                    ->update($data);

                return;
            } catch (QueryException $e) {
                if ($this->isDeadlock($e) && $i < $attempts - 1) {
                    usleep($backoffMs * 1000 * ($i + 1));
                    continue;
                }

                throw $e;
            }
        }
    }

    private function updateGameElementByIdWithRetry(int $id, array $data, int $attempts = 3, int $backoffMs = 100): void
    {
        for ($i = 0; $i < $attempts; $i++) {
            try {
                \DB::table('game_elements')
                    ->where('id', $id)
                    ->update($data);

                return;
            } catch (QueryException $e) {
                if ($this->isDeadlock($e) && $i < $attempts - 1) {
                    usleep($backoffMs * 1000 * ($i + 1));
                    continue;
                }

                throw $e;
            }
        }
    }

    private function incrementWinCountWithRetry(int $gameId, int $elementId, int $attempts = 3, int $backoffMs = 100): void
    {
        for ($i = 0; $i < $attempts; $i++) {
            try {
                \DB::table('game_elements')
                    ->where('game_id', $gameId)
                    ->where('element_id', $elementId)
                    ->increment('win_count');

                return;
            } catch (QueryException $e) {
                if ($this->isDeadlock($e) && $i < $attempts - 1) {
                    usleep($backoffMs * 1000 * ($i + 1));
                    continue;
                }

                throw $e;
            }
        }
    }

    private function incrementWinCountByIdWithRetry(int $id, int $attempts = 3, int $backoffMs = 100): void
    {
        for ($i = 0; $i < $attempts; $i++) {
            try {
                \DB::table('game_elements')
                    ->where('id', $id)
                    ->increment('win_count');

                return;
            } catch (QueryException $e) {
                if ($this->isDeadlock($e) && $i < $attempts - 1) {
                    usleep($backoffMs * 1000 * ($i + 1));
                    continue;
                }

                throw $e;
            }
        }
    }

    private function isDeadlock(QueryException $e): bool
    {
        $sqlState = $e->getCode(); // MySQL deadlock: 40001; sometimes driver-specific 1213 message
        return $sqlState === '40001' || str_contains($e->getMessage(), 'Deadlock') || str_contains($e->getMessage(), '1213');
    }

    /**
     * @return Game1V1Round|null
     */
    public function batchUpdateGameRounds(Game $game, array $votes)
    {
        $tStart = microtime(true);
        logger('batchUpdateGameRounds.start', [
            'game_id' => $game->id,
            'votes_count' => count($votes),
        ]);

        $this->validateBatchVotes($game, $votes);

        $tAfterValidate = microtime(true);
        logger('batchUpdateGameRounds.after_validate', [
            'elapsed_ms' => round(($tAfterValidate - $tStart) * 1000, 2),
        ]);

        ksort($votes);
        $lock = Locker::lockUpdateGameElement($game);
        $tBeforeLock = microtime(true);
        $lock->block(10);
        $tAfterLock = microtime(true);
        logger('batchUpdateGameRounds.lock_acquired', [
            'wait_ms' => round(($tAfterLock - $tBeforeLock) * 1000, 2),
        ]);

        try {
            $lastCreatedRound = null;
            $lastRound = $game->game_1v1_rounds()->latest('id')->first();

            // Preload all game_elements once by game_id
            $gameElements = \DB::table('game_elements')
                ->where('game_id', $game->id)
                ->get()
                ->keyBy('element_id');

            $gameElementStates = [];
            $originalStates = [];
            foreach ($gameElements as $elementId => $row) {
                $state = [
                    'id' => $row->id,
                    'element_id' => $elementId,
                    'win_count' => (int) $row->win_count,
                    'is_eliminated' => (bool) $row->is_eliminated,
                    'is_ready' => (bool) $row->is_ready,
                ];
                $gameElementStates[$elementId] = $state;
                $originalStates[$elementId] = $state;
            }

            logger('batchUpdateGameRounds.element_map_loaded', [
                'game_id' => $game->id,
                'count' => count($gameElementStates),
            ]);

            // 1. 判斷目前的 Stage (第幾輪)
            $stageCount = $game->game_1v1_rounds()->where('current_round', 1)->count();

            // 2. 初始化狀態變數
            if ($lastRound === null) {
                $stage = 1;
                $remain = $game->element_count;
                $matchIndex = 0;
                $matchesInStage = $this->calculateMatchesForStage($stage, $remain);
            } else {
                $remain = $lastRound->remain_elements;
                if ($lastRound->current_round >= $lastRound->of_round) {
                    $stage = $stageCount + 1;
                    $matchIndex = 0;
                    $matchesInStage = $this->calculateMatchesForStage($stage, $remain);
                } else {
                    $stage = $stageCount > 0 ? $stageCount : 1;
                    $matchIndex = $lastRound->current_round;
                    $matchesInStage = $lastRound->of_round;
                }
            }

            logger('batchUpdateGameRounds.state_init', [
                'game_id' => $game->id,
                'last_round_id' => $lastRound?->id,
                'last_round_current' => $lastRound?->current_round,
                'last_round_of_round' => $lastRound?->of_round,
                'last_round_remain' => $lastRound?->remain_elements,
                'stage_count' => $stageCount,
                'stage' => $stage,
                'remain' => $remain,
                'match_index' => $matchIndex,
                'matches_in_stage' => $matchesInStage,
                'votes_count' => count($votes),
            ]);

            // 準備批次寫入的陣列
            $roundsToInsert = [];
            $now = now();
            $voteCountIncrement = 0;
            $stats = [
                'winner_updates' => 0,
                'loser_updates' => 0,
                'ready_resets' => 0,
            ];

            // 直接執行迴圈，每一行 SQL 執行完就會自動釋放 Row Lock
            foreach ($votes as $vote) {
                $loopStart = microtime(true);
                $winnerId = $vote['winner_id'];
                $loserId = $vote['loser_id'];

                if (!isset($gameElementStates[$winnerId]) || !isset($gameElementStates[$loserId])) {
                    throw new \RuntimeException("game_element row missing for winner {$winnerId} or loser {$loserId} in game {$game->id}");
                }

                // mutate in-memory state
                $gameElementStates[$winnerId]['win_count'] += 1;
                $gameElementStates[$winnerId]['is_ready'] = false;

                $gameElementStates[$loserId]['is_eliminated'] = true;
                $gameElementStates[$loserId]['is_ready'] = false;

                $remain--;
                $matchIndex++;
                $voteCountIncrement++;

                if ($matchIndex > $matchesInStage) {
                    $stage++;
                    $matchIndex = 1;
                    $matchesInStage = $this->calculateMatchesForStage($stage, $remain + 1);
                }

                $isEndOfRound = ($matchIndex === $matchesInStage);

                $stats['winner_updates']++;
                $stats['loser_updates']++;

                if ($isEndOfRound) {
                    foreach ($gameElementStates as &$state) {
                        if (!$state['is_eliminated']) {
                            $state['is_ready'] = true;
                        }
                    }
                    unset($state);
                    $stats['ready_resets']++;
                }

                // 收集 Round 資料，稍後一次寫入
                $roundsToInsert[] = [
                    'game_id' => $game->id,
                    'current_round' => $matchIndex,
                    'of_round' => $matchesInStage,
                    'remain_elements' => $remain,
                    'winner_id' => $winnerId,
                    'loser_id' => $loserId,
                    'created_at' => $now,
                    'updated_at' => $now,
                ];
                logger('prepared round data', end($roundsToInsert));

                logger('batchUpdateGameRounds.vote_processed', [
                    'game_id' => $game->id,
                    'winner_id' => $winnerId,
                    'loser_id' => $loserId,
                    'stage' => $stage,
                    'match_index' => $matchIndex,
                    'matches_in_stage' => $matchesInStage,
                    'remain' => $remain,
                    'is_end_of_round' => $isEndOfRound,
                    'elapsed_ms' => round((microtime(true) - $loopStart) * 1000, 2),
                ]);
            }

            // Apply consolidated updates back to DB (only changed rows)
            $elementUpdates = [];
            foreach ($gameElementStates as $elementId => $state) {
                $orig = $originalStates[$elementId];
                $data = [];
                if ($state['win_count'] !== $orig['win_count']) {
                    $data['win_count'] = $state['win_count'];
                }
                if ($state['is_eliminated'] !== $orig['is_eliminated']) {
                    $data['is_eliminated'] = $state['is_eliminated'];
                }
                if ($state['is_ready'] !== $orig['is_ready']) {
                    $data['is_ready'] = $state['is_ready'];
                }

                if (!empty($data)) {
                    $elementUpdates[] = ['id' => $state['id'], 'data' => $data];
                }
            }

            foreach ($elementUpdates as $update) {
                $this->updateGameElementByIdWithRetry($update['id'], $update['data']);
            }

            logger('batchUpdateGameRounds.elements_updated', [
                'game_id' => $game->id,
                'rows' => count($elementUpdates),
            ]);

            // 使用 chunk 防止一次寫入過多導致 SQL 長度過長
            if (!empty($roundsToInsert)) {
                foreach (array_chunk($roundsToInsert, 500) as $chunk) {
                    $chunkStart = microtime(true);
                    Game1V1Round::insert($chunk);
                    logger('batchUpdateGameRounds.insert_chunk', [
                        'size' => count($chunk),
                        'elapsed_ms' => round((microtime(true) - $chunkStart) * 1000, 2),
                    ]);
                }

                $lastData = end($roundsToInsert);
                $lastCreatedRound = Game1V1Round::where('game_id', $game->id)
                    ->where('current_round', $lastData['current_round'])
                    ->where('of_round', $lastData['of_round'])
                    ->where('remain_elements', $lastData['remain_elements'])
                    ->orderByDesc('id')
                    ->first();
            }

            // 最後只更新一次 Game 表 (減少 Row Lock 競爭)
            if ($voteCountIncrement > 0) {
                \DB::table('games')
                    ->where('id', $game->id)
                    ->increment('vote_count', $voteCountIncrement);

                // 更新 user_id (如果是登入用戶)
                if (request()->user()) {
                    \DB::table('games')
                        ->where('id', $game->id)
                        ->update(['user_id' => request()->user()->id]);
                }
            }

            $lock->release();
            logger('batchUpdateGameRounds.done', [
                'game_id' => $game->id,
                'votes_count' => count($votes),
                'winner_updates' => $stats['winner_updates'],
                'loser_updates' => $stats['loser_updates'],
                'ready_resets' => $stats['ready_resets'],
                'total_ms' => round((microtime(true) - $tStart) * 1000, 2),
            ]);
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

        // remove won_at and lost_at that not match
        GameRoomUserBet::where('game_room_id', $gameRoom->id)
            ->where('won_at', null)
            ->where('lost_at', null)
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

    public function getUserLastGame(Post $post, ?User $user): ?Game
    {
        if(!$user){
            return null;
        }

        $game = Game::where('post_id', $post->id)
            ->where('user_id', $user?->id)
            ->orderByDesc('id')
            ->select(['completed_at', 'serial', 'updated_at',  'vote_count', 'element_count'])
            ->first();

        if($game && !$game->completed_at){
            return $game;
        }

        return null;
    }
}
