<?php

namespace App\Services;

use App\Enums\PostAccessPolicy;
use DB;
use App\Models\Tag;

class TagService
{
    public function get(?string $name)
    {

        $query = Tag::join('post_tags', 'tags.id', '=', 'post_tags.tag_id')
            ->whereHas('posts.post_policy', function ($query) {
                $query->where('access_policy', PostAccessPolicy::PUBLIC );
            })
            ->groupBy('tags.id')
            ->orderByRaw('count(tags.id) desc')
            ->select('tags.name', DB::raw('count(tags.id) as count'))
            ->limit(5);

        if (!is_null($name) && $name !== '') {
            $tags = $query->where('name', 'like', "%$name%");
        }

        $tags = $query->get();

        foreach ($tags as $tag) {
            // hide the count for now
            $tag->count = 0;
            // $tag->count = max(((int) ($tag->count / 10)) * 10, 10);
        }

        return $tags;
    }
}