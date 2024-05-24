<?php


namespace App\Services;


use App\Helper\Locker;
use App\Http\Resources\Game\GameResultResource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
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
            $game->elements()->attach($element);
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
        return optional($game->game_1v1_rounds()
            ->where('remain_elements', 1)
            ->first())->winner;
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
        $winnerRank = $rankService->getRankPosition($game->post, $winner);

        $gameResult = GameResultResource::collection($rounds)
            ->additional([
                'winner' => $winner,
                'winner_rank' => $winnerRank,
                'statistics' => [
                    'timeline' => $this->getGameTimeline($game),
                    'game_time' => $game->created_at->diffInSeconds($game->completed_at),
                    'winner_id' => $winner->id,
                    'winner_global_rank' => $winnerRank
                ]
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
            
            \DB::transaction(function () use ($game, $winnerId, $loserId) {
                // update winner
                $gameElement = $game->game_elements()
                    ->where('element_id', $winnerId)
                    ->where('is_eliminated', false)
                    ->first();
                if($gameElement){
                    $gameElement->update([
                        'win_count' => $gameElement->win_count + 1
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
                        'is_eliminated' => true
                    ]);
                    \Log::debug('game element updated', ['game_id' => $game->id, 'element_id' => $loserId, 'is_eliminated' => $gameElement->is_eliminated]);
                }else{
                    \Log::error('game element not found', ['game_id' => $game->id, 'element_id' => $loserId]);
                    throw new \Exception('game element not found');
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
}
