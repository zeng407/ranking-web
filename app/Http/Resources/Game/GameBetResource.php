<?php

namespace App\Http\Resources\Game;

use App\Models\GameRoomUserBet;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin GameRoomUserBet
 */
class GameBetResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $hash = game_round_hash($this->current_round, $this->of_round, $this->remain_elements, [$this->winner_id, $this->loser_id]);
        return [
            'hash' => $hash,
            'current_round' => $this->current_round,
            'of_round' => $this->of_round,
            'remain_elements' => $this->remain_elements,
            'winner_id' => $this->winner_id,
            'loser_id' => $this->loser_id,
        ];
    }
}
