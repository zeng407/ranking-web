<?php

namespace App\Http\Resources\Game;

use App\Helper\CacheService;
use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin GameRoom
 */
class HostGameRoomResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $gameService = new GameService;
        return [
            'online_users' => $gameService->getChannelConnectionCount($this->resource),
            'serial' => $this->serial,
            'is_game_completed' => $this->game->completed_at !== null,
            'ranks' => CacheService::rememberGameBetRank($this->resource)
        ];
    }
}
