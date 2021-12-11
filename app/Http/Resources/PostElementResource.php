<?php

namespace App\Http\Resources;

use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\GameElement;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameResource
 * @package App\Http\Resources
 * @mixin Element
 */
class PostElementResource extends JsonResource
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
            'id' => $this->id,
            'source_url' => $this->source_url,
            'thumb_url' => $this->thumb_url,
            'title' => $this->title,
            'type' => $this->type,
            'video_start_second' => $this->video_end_second,
            'video_end_second' => $this->video_end_second
        ];
    }
}
