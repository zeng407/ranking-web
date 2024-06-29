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
        if($request->query('q') == 'rank'){
            return [
                'total_users' => $this->users()->count(),
                'serial' => $this->serial,
                'is_game_completed' => $this->game->completed_at !== null,
                'ranks' => CacheService::rememberGameBetRank($this->resource)
            ];
        }

        $round = $this->game->game_1v1_rounds()->orderByDesc('id')->first();
        if($this->game->completed_at){
            $elements = [];
        }else{
            $elementsId = explode(',', $this->game->candidates);
            //todo: query elements one time instead of two times
            $elements = [
                $this->game->elements()->find($elementsId[0]),
                $this->game->elements()->find($elementsId[1]),
            ];
        }
        $gameRoomUser = (new GameService)->getGameRoomUser($this->resource, $request);
        $bet = $gameRoomUser->bets()->where('game_room_id', $this->id)->latest('id')->first();
        return [
            'user' => GameRoomUserResource::make($gameRoomUser),
            'total_users' => $this->users()->count(),
            'serial' => $this->serial,
            'last_round' => $round ? [
                'current_round' => $round->current_round,
                'of_round' => $round->of_round,
                'remain_elements' => $round->remain_elements,
                'winner' => GameElementResource::make($round->winner),
                'loser' => GameElementResource::make($round->loser),
            ] : [],
            'current_round' => $elements ? GameRoundResource::make($this->game, $elements) : null,
            'bet' => GameBetResource::make($bet),
            'is_game_completed' => $this->game->completed_at !== null,
            'ranks' => CacheService::rememberGameBetRank($this->resource)
        ];
    }
}
