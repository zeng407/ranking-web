<?php

namespace App\Jobs;

use App\Helper\CacheService;
use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Cache;

class UpdateGameBet implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected GameRoom $gameRoom;
    protected array $conditions;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(GameRoom $gameRoom, array $conditions)
    {
        $this->gameRoom = $gameRoom;
        $this->conditions = $conditions;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle(GameService $gameService)
    {
        $gameService->updateGameBet($this->gameRoom, $this->conditions['winner_id'], $this->conditions['loser_id'], $this->conditions);

        CacheService::putJobCacheUpdateGameRoomRank($this->gameRoom);
        UpdateGameRoomRank::dispatch($this->gameRoom);
    }
}
