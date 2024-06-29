<?php

namespace App\Http\Resources\Game;

use App\Models\GameRoomUser;
use App\Services\GameService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin GameRoomUser
 */
class GameRoomUserResource extends JsonResource
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
            'user_id' => md5($this->id.':'.$this->anonymous_id),
            'name' => $this->nickname,
            'score' => $this->score,
            'rank' => $this->rank,
            'accuracy' => $this->accuracy,
            'total_played' => $this->total_played,
            'total_correct' => $this->total_correct,
            'combo' => $this->combo,
        ];
    }
}
