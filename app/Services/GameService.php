<?php


namespace App\Services;


use App\Helper\CacheService;
use App\Helper\Locker;
use App\Helper\SerialGenerator;
use App\Http\Resources\Game\GameResultResource;
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

    public function createGame(Post $post, $elementCount): Game
    {
        /** @var Game $game */
        $game = $post->games()->create([
            'serial' => Uuid::uuid1()->toString(),
            'element_count' => $elementCount
        ]);

        // pick random elements
        $elements = $post->elements()
            ->inRandomOrder()
            ->take($game->element_count)
            ->get();

        $elements->each(function (Element $element) use ($game) {
            $game->elements()->attach($element, [
                'is_ready' => true
            ]);
        });

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
    }

    public function updateGameRoomUserBetScore(GameRoomUser $gameRoomUser)
    {
        $totalPlayed = $gameRoomUser->bets()->count();
        $totalCorrect = $gameRoomUser->bets()->whereNotNull('won_at')->count();
        $accuracy = $totalPlayed > 0 ? $totalCorrect / $totalPlayed * 100 : 0;
        $score = $gameRoomUser->bets()->sum('score') + config('setting.default_bet_score');
        $lastBet = $gameRoomUser->bets()
            ->latest('id')
            ->select(['last_combo','won_at'])
            ->first();
        $combo = 0;
        if($lastBet){
            $combo = $lastBet->won_at ? ($lastBet->last_combo + 1) : 0;
        }

        $gameRoomUser->update([
            'combo' => $combo,
            'score' => $score,
            'accuracy' => $accuracy,
            'total_played' => $totalPlayed,
            'total_correct' => $totalCorrect
        ]);
    }
}
