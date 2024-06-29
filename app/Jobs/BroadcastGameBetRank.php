<?php

namespace App\Jobs;

use App\Helper\CacheService;
use App\Models\GameRoom;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Broadcasting\ShouldBroadcastNow;
use Illuminate\Broadcasting\Channel;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class BroadcastGameBetRank implements ShouldBroadcastNow
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
        logger('BroadcastGameBetRank', [$this->gameRoom->serial]);
    }

    public function broadcastWith()
    {
        // save rank to cache
        $data = CacheService::rememberGameBetRank($this->gameRoom, true);
        return $data;
    }

    public function broadcastOn()
    {
        return new Channel('game-room.' . $this->gameRoom->serial);
    }

    public function broadcastAs()
    {
        return 'GameBetRank';
    }
}
