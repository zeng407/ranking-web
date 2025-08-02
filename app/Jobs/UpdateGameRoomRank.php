<?php

namespace App\Jobs;

use App\Helper\CacheService;
use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Cache;

class UpdateGameRoomRank implements ShouldQueue, ShouldBeUnique
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
    public function handle(GameService $gameService)
    {
        // Remove from waiting job
        CacheService::pullJobCacheUpdateGameRoomRank($this->gameRoom);

        $this->gameRoom->users()->each(function ($user) use ($gameService) {
            $gameService->updateGameRoomUserBetScore($user);
        });

        $rank = 1;
        $this->gameRoom->users()->orderByDesc('score')->each(function ($user) use (&$rank) {
            $user->update(['rank' => $rank]);
            $rank++;
        });

        broadcast(new BroadcastGameBetRank($this->gameRoom));

        // Check waiting job
        if ($this->shouldReDispatch()) {
            ReUpdateGameRoomRank::dispatch($this->gameRoom)->delay(now()->addSeconds(5));
        }else{
            // Remove from processing job
            CacheService::pullUpdatingGameRoomRank($this->gameRoom);
        }
    }

    public function uniqueId()
    {
        return $this->gameRoom->serial;
    }

    protected function shouldReDispatch()
    {
        return CacheService::hasJobCacheUpdateGameRoomRank($this->gameRoom);
    }
}
