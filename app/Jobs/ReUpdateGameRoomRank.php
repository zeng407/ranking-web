<?php

namespace App\Jobs;

use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class ReUpdateGameRoomRank implements ShouldQueue
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
        $this->onQueue('game_room');
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        logger('ReUpdateGameRoomRank', [$this->gameRoom->serial]);
        UpdateGameRoomRank::dispatch($this->gameRoom);
    }
}
