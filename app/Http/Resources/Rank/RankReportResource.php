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
            'element' => [
                'title' => $this->element->title,
                'type' => $this->element->type,
                'id' => $this->element->id,
                'video_id' => $this->element->video_id,
                'source_url' => $this->element->source_url,
                'video_source' => $this->element->video_source,
                'thumb_url' => $this->element->thumb_url,
                'imgur_url' => $this->element->imgur_url,
                'lowthumb_url' => $this->element->lowthumb_url,
                'mediumthumb_url' => $this->element->mediumthumb_url,
            ]
        ];
    }
}
