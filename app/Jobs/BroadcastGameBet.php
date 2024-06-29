<?php

namespace App\Jobs;

use App\Helper\CacheService;
use App\Http\Resources\Game\GameRoomVoteResource;
use App\Models\GameRoom;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Broadcasting\Channel;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class BroadcastGameBet implements ShouldBroadcast
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public GameRoom $gameRoom;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(GameRoom $gameRoom)
    {
        $this->gameRoom = $gameRoom;
        logger('BroadcastGameBet', [$this->gameRoom->serial]);
    }

    public function broadcastWith()
    {
        $data = GameRoomVoteResource::make($this->gameRoom)->toArray(request());
        return $data;
    }

    public function broadcastOn()
    {
        return new Channel('game-room.' . $this->gameRoom->serial.'.game-serial.'.$this->gameRoom->game->serial);
    }

    public function broadcastAs()
    {
        return 'GameBet';
    }
}
