<?php

namespace App\Events;

use App\Models\Element;
use App\Models\Game;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;
use App\Models\User;



class GameComplete
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    public Game $game;
    public Element $champion;
    public ?User $user;
    public string $anonymousId;

    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(?User $user, string $anonymousId, Game $game, Element $champion)
    {
        logger('[GameComplete] event fired', [
            'game' => $game->id,
            'champion' => $champion->id,
            'user' => $user?->id,
            'anonymousId' => $anonymousId
        ]);
        $this->game = $game;
        $this->anonymousId = $anonymousId;
        $this->champion = $champion;
        $this->user = $user;
    }

}
