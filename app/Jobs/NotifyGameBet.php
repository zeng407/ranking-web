<?php

namespace App\Jobs;

use App\Helper\CacheService;
use App\Http\Resources\Game\GameRoomVoteResource;
use App\Models\GameRoom;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class NotifyGameBet implements ShouldQueue, ShouldBeUnique
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected GameRoom $gameRoom;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(GameRoom $gameRoom)
    {
        $this->gameRoom = $gameRoom;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        $gameRoom = $this->gameRoom;
        // pull cache
        CacheService::pullJobCacheUpdateGameBet($gameRoom);

        // broadcast the game room rank
        broadcast(new BroadcastGameBet($gameRoom));

        if($this->shouldReDispatch($gameRoom)){
            ReNotifyGameBet::dispatch($gameRoom)->delay(now()->addSeconds(3));
        }
    }

    public function shouldReDispatch(GameRoom $gameRoom)
    {
        return CacheService::hasJobCacheUpdateGameBet($gameRoom);
    }

    public function uniqueId()
    {
        return $this->gameRoom->serial;
    }
}
