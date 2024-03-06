<?php

namespace App\Http\Resources\MyPost;

use App\Enums\PostAccessPolicy;
use App\Models\Post;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class PostResource
 * @package App\Http\Resources
 * @mixin Post
 */
class PostResource extends JsonResource
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
            'serial' => $this->serial,
            'description' => $this->description,
            'policy' => $this->post_policy->access_policy,
            'play_count' => $this->games()->count(),
            'created_at' => $this->created_at,
            'tags' => $this->tags->pluck('name')->toArray(),
        ];
    }
}
