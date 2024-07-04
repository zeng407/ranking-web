<?php

namespace App\Http\Resources\Game;

use App\Helper\CacheService;
use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin GameRoom
 */
class GameRoomResource extends JsonResource
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
        if($request->query('q') == 'rank'){
            return [
                'online_users' => $gameService->getChannelConnectionCount($this->resource),
                'serial' => $this->serial,
                'is_game_completed' => $this->game->completed_at !== null,
                'ranks' => CacheService::rememberGameBetRank($this->resource)
            ];
        }

        $elements = $gameService->getCurrentElements($this->game);
        $gameRoomUser = $gameService->getGameRoomUser($this->resource, $request);
        $bet = $gameRoomUser->bets()->where('game_room_id', $this->id)->latest('id')->first();
        return [
            'user' => GameRoomUserResource::make($gameRoomUser),
            'online_users' => $gameService->getChannelConnectionCount($this->resource),
            'serial' => $this->serial,
            'current_round' => $elements ? GameRoundResource::make($this->game, $elements) : null,
            'bet' => GameBetResource::make($bet),
            'is_game_completed' => $this->game->completed_at !== null,
            'ranks' => CacheService::rememberGameBetRank($this->resource)
        ];
    }
}
