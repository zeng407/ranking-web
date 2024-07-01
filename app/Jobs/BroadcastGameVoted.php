<?php

namespace App\Jobs;

use App\Models\Element;
use App\Models\GameRoom;
use Illuminate\Broadcasting\Channel;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Broadcasting\ShouldBroadcastNow;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class BroadcastGameVoted implements ShouldBroadcastNow
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public GameRoom $gameRoom;
    public Element $winner;
    public Element $loser;
    public array $nextRound;
    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(GameRoom $gameRoom, Element $winner, Element $loser, array $nextRound)
    {
        $this->gameRoom = $gameRoom;
        $this->winner = $winner;
        $this->loser = $loser;
        $this->nextRound = $nextRound;
        logger('BroadcastGameVoted', [$this->gameRoom->serial, $this->winner->id, $this->loser->id]);
    }

    public function broadcastWith()
    {
        return [
            'winner_id' => $this->winner->id,
            'loser_id' => $this->loser->id,
            'next_round' => $this->nextRound ?: null,
        ];
    }

    public function broadcastOn()
    {
        return new Channel('game-room.' . $this->gameRoom->serial);
    }

    public function broadcastAs()
    {
        return 'NotifyVoted';
    }
}
