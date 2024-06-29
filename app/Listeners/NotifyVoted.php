<?php

namespace App\Listeners;

use App\Events\GameElementVoted;
use App\Http\Resources\Game\GameRoundResource;
use App\Jobs\BroadcastGameVoted;
use App\Jobs\UpdateGameBet;

class NotifyVoted
{
    /**
     * Create the event listener.
     *
     * @return void
     */
    public function __construct()
    {
        //
    }

    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(GameElementVoted $event)
    {
        $game = $event->game;
        $round = $event->gameRound;
        if(!$game->game_room){
            return;
        }
        $candidates = explode(',', $game->candidates);
        if(count($candidates) === 2){
            $elements = [
                $game->elements()->find($candidates[0]),
                $game->elements()->find($candidates[1]),
            ];
            $data = GameRoundResource::make($game, $elements)->toArray(request());
        }else{
            $data = [];
        }

        broadcast(new BroadcastGameVoted($game->game_room, $round->winner, $round->loser, $data));
        UpdateGameBet::dispatch($game->game_room, [
            'winner_id' => $round->winner_id,
            'loser_id' => $round->loser_id,
            'current_round' => $round->current_round,
            'of_round' => $round->of_round,
            'remain_elements' => $round->remain_elements,
            'is_game_complete' => $game->completed_at !== null,
        ]);

    }
}
