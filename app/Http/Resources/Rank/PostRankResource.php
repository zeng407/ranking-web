<?php

namespace App\Http\Resources\Rank;

use App\Enums\RankType;
use App\Models\Post;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class PostRankResource
 * @package App\Http\Resources\Rank
 * @mixin Post
 */
class PostRankResource extends JsonResource
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
        ];
    }
}
