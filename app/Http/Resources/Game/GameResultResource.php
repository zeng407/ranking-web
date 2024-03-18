<?php

namespace App\Http\Resources\Game;

use App\Models\Game1V1Round;
use App\Services\RankService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameResultResource
 * @package App\Http\Resources
 * @mixin Game1V1Round
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
        /** @var RankService */
        $service = app(RankService::class);
        return [
            'loser' => GameElementResource::make($this->loser),
            'rank' => $service->getRankPosition($this->game->post, $this->loser)
        ];
    }

}
