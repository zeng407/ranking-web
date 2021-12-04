<?php

namespace App\Http\Resources;

use App\Models\Game;
use App\Models\Game1V1Round;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameResource
 * @package App\Http\Resources
 * @mixin Game
 */
class GameResultResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $lastRound = $this->game_1v1_rounds()
            ->where('')
            ->first();
        return [
            'title' => $this->post->title,
            'winner' => GameElementResource::make($winner)
        ];
    }
}
