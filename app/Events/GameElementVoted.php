<?php

namespace App\Events;

use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class GameElementVoted
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    public Game $game;
    public Game1V1Round $gameRound;



    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(Game $game, Game1V1Round $gameRound)
    {
        $this->game = $game;
        $this->gameRound = $gameRound;
    }

}
