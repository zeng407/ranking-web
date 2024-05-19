<?php

namespace App\Events;

use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use Illuminate\Broadcasting\InteractsWithSockets;
use Illuminate\Foundation\Events\Dispatchable;
use Illuminate\Queue\SerializesModels;
use App\Models\User;



class GameComplete
{
    use Dispatchable, InteractsWithSockets, SerializesModels;

    public Game1V1Round $gameRound;
    public Game $game;
    public Post $post;
    public ?User $user;
    public string $anonymousId;
    public string $candidates;

    /**
     * Create a new event instance.
     *
     * @return void
     */
    public function __construct(?User $user, string $anonymousId, Game1V1Round $gameRound, string $candicates)
    {
        logger('[GameComplete] event fired', [
            'game' => $gameRound->game_id,
            'champion' => $gameRound->winner_id,
            'user' => $user?->id,
            'anonymousId' => $anonymousId
        ]);

        $this->anonymousId = $anonymousId;
        $this->gameRound = $gameRound;
        $this->user = $user;
        $this->game = $gameRound->game;
        $this->post = $gameRound->game->post;
        $this->candidates = $candicates;
    }

}
