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
        return [
            'total_users' => $this->users()->count(),
            'serial' => $this->serial,
            'is_game_completed' => $this->game->completed_at !== null,
            'ranks' => CacheService::rememberGameBetRank($this->resource)
        ];
    }
}
