<?php

namespace App\Events;

use App\Models\Element;
use App\Models\Game;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;

class GameElementVoted
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    public Element $element;
    public Game $game;
    public bool $isFinal;

    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(Game $game, Element $element, $isFinal)
    {
        \Log::debug("GameElementVoted");
        $this->element = $element;
        $this->game = $game;
        $this->isFinal = $isFinal;
    }

}
