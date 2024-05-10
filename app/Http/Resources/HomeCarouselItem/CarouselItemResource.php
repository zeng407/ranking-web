<?php

namespace App\Http\Resources\HomeCarouselItem;

use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin \App\Models\HomeCarouselItem
 */
class CarouselItemResource extends JsonResource
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
            'title' => $this->title,
            'description' => $this->description,
            'image_url' => $this->image_url,
            'video_url' => $this->video_url,
            'position' => $this->position,
            'type' => $this->type,
            'video_source' => $this->video_source,
            'video_id' => $this->video_id,
            'video_start_second' => $this->video_start_second,
        ];
    }
}
