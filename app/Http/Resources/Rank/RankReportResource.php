<?php

namespace App\Http\Resources\Rank;

use Illuminate\Http\Resources\Json\JsonResource;

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
        if($this->resource == null){
            return [];
        }
        return [
            'rank' => $this->rank,
            'win_rate' => (string) round($this->win_rate,1),
            'date' => today()->toDateString(),
        ];
    }
}
