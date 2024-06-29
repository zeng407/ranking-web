<?php

namespace App\Jobs;

use App\Http\Resources\Game\GameRoundResource;
use App\Models\Game;
use App\Models\GameRoom;
use Illuminate\Broadcasting\Channel;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Broadcasting\ShouldBroadcastNow;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class BroadcastGameRoomRefresh implements ShouldBroadcastNow
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
    }

    public function broadcastOn()
    {
        return new Channel('game-room.' . $this->gameRoom->serial);
    }

    public function broadcastAs()
    {
        return 'GameRoomRefresh';
    }

    public function broadcastWith()
    {
        $game = $this->gameRoom->game()->first();
        $candidates = explode(',', $game->candidates);
        if(count($candidates) === 2){
            $elements = [
                $game->elements()->find($candidates[0]),
                $game->elements()->find($candidates[1]),
            ];
            $data = GameRoundResource::make($game, $elements)->toArray(request());
        }else{
            $data = [];
        }
        return [
            'next_round' => $data
        ];
    }
}
