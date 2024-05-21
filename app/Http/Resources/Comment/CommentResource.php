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
        if($this->anonymous_mode) {
            $nickname = config('setting.anonymous_nickname');
            $avatar_url = null;
        }else{
            $nickname = $this->nickname;
            $avatar_url = $this->user?->avatar_url;
        }

        return [
            'id' => $this->id,
            'content' => $this->content,
            'created_at' => $this->created_at,
            'edited_at' => $this->edited_at,
            'nickname' => $nickname,
            'deleted_at' => $this->deleted_at,
            'avatar_url' => $avatar_url,
            'champions' => $this->getChampions(),
        ];
    }
}
