<?php

namespace App\Http\Resources\MyPost;

use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\GameElement;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class PostElementResource
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
            'mediumthumb_url' => $this->mediumthumb_url,
            'lowthumb_url' => $this->getLowThumbUrl(),
            'thumb_url2' => $this->imgur_image?->link,
            'title' => $this->title,
            'type' => $this->type,
            'video_source' => $this->video_source,
            'video_id' => $this->video_id,
            'video_duration_second' => $this->video_duration_second,
            'video_start_second' => $this->video_start_second,
            'video_end_second' => $this->video_end_second,
            'created_at' => $this->created_at,
            'rank' => $this->rank_reports->first(),
        ];
    }
}
