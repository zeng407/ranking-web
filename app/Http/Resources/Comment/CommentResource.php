<?php

namespace App\Http\Resources\Comment;

use App\Models\Comment;
use App\Models\Post;
use Illuminate\Http\Resources\Json\JsonResource;
use App\Services\PostService;

/**
 * @mixin Comment
 */
class CommentResource extends JsonResource
{
    public function toArray($request)
    {
        /** @var PostService */
        $postService = app(PostService::class);

        return [
            'id' => $this->id,
            'content' => $this->content,
            'created_at' => $this->created_at,
            'edited_at' => $this->edited_at,
            'nickname' => $this->nickname,
            'deleted_at' => $this->deleted_at,
            'avatar_url' => $this->user?->avatar_url,
            'champions' => $this->getChampions(),
        ];
    }
}
