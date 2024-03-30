<?php


namespace App\Services;


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

        $gameResult = GameResultResource::collection($rounds)
            ->additional([
                'winner' => $winner,
                'winner_rank' => $rankService->getRankPosition($game->post, $winner)
            ])
            ->toResponse($request)->getData();

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
            $ofRound = (int) ceil($lastRound->remain_elements / 2);
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
        \Log::info('saving game : ' . $game->id, $data);

        \DB::transaction(function () use ($game, $winnerId, $loserId) {
            // update winner
            $gameElement = $game->elements()
                ->where('element_id', $winnerId)
                ->where('is_eliminated', false)
                ->first();
            if($gameElement){
                $gameElement->update([
                    'win_count' => $gameElement->win_count + 1
                ]);
            }

            // update loser
            $gameElement = $game->elements()
                ->where('element_id', $loserId)
                ->where('is_eliminated', false)
                ->first();
            if($gameElement){
                $gameElement->update([
                    'is_eliminated' => true
                ]);
            }
        });

        return $game->game_1v1_rounds()->create($data);
    }

    public function createVotedChampion(?User $user, string $anonymousId, Game $game, Element $element)
    {
        $title = $element->title ?? '';
        if($user){
            UserGameResult::create([
                'user_id' => $user->id,
                'game_id' => $game->id,
                'champion_id' => $element->id,
                'champion_name' => $title
            ]);
        }else{
            UserGameResult::create([
                'anonymous_id' => $anonymousId,
                'game_id' => $game->id,
                'champion_id' => $element->id,
                'champion_name' => $title
            ]);
        }
    }
}
