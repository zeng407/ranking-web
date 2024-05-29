<?php

namespace App\Http\Resources\Rank;

use App\Http\Resources\Game\GameElementResource;
use App\Models\RankReportHistory;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class RankReportHistoryResource
 * @package App\Http\Resources\Rank
 * @mixin RankReportHistory
 */
class WeeklyRankResource extends JsonResource
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
            'rank' => $this->rank,
            'win_rate' => $this->win_rate,
            'date' => $this->start_date,
            'element' => GameElementResource::make($this->element),
        ];
    }
}
