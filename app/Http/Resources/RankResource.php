<?php

namespace App\Http\Resources;

use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameResource
 * @package App\Http\Resources
 * @mixin Post
 */
class RankResource extends JsonResource
{
    protected $elements;

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
            'description' => $this->description
        ];
    }
}
