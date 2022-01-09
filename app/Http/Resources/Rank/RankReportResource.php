<?php

namespace App\Http\Resources\Rank;

use App\Http\Resources\Game\GameElementResource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Http\Resources\Json\JsonResource;
use Illuminate\Http\Resources\Json\PaginatedResourceResponse;
use Illuminate\Http\Resources\Json\ResourceCollection;
use Illuminate\Http\Resources\Json\ResourceResponse;

/**
 * Class RankReportResource
 * @package App\Http\Resources\Rank
 * @mixin RankReport
 */
class RankReportResource extends JsonResource
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
            'element' => GameElementResource::make($this->element),
            'final_win_position' => $this->final_win_position,
            'final_win_rate' => (int)$this->final_win_rate,
            'win_position' => $this->win_position,
            'win_rate' => (int)$this->win_rate,
            'rank' => $this->rank
        ];
    }
}
