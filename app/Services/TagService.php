<?php

namespace App\Services;

use App\Enums\PostAccessPolicy;
use App\Helper\CacheService;
use DB;
use App\Models\Tag;

class TagService
{
    public function get(?string $name = null, int $limit = 10)
    {
        $query = Tag::join('post_tags', 'tags.id', '=', 'post_tags.tag_id')
            ->whereHas('posts.post_policy', function ($query) {
                $query->where('access_policy', PostAccessPolicy::PUBLIC);
            })
            ->groupBy('tags.id')
            ->orderByRaw('count(tags.id) desc')
            ->select('tags.name', DB::raw('count(tags.id) as count'))
            ->limit($limit);

        if (!is_null($name) && $name !== '') {
            $tags = $query->where('name', 'like', "%$name%");
        }

        $tags = $query->get();

        return $tags;
    }

    public function getHotTags(int $limit = 10)
    {
        // get the hot psots
        $posts = CacheService::rememberPosts(request(), 'hot');

        $tags = [];
        foreach ($posts as $post) {
            foreach ($post['tags'] as $tag) {
                if(!in_array($tag, $tags)) {
                    $tags[] = $tag;
                }
            }
        }
        $tags = Tag::whereIn('name', $tags)
            ->withCount('posts')
            ->orderBy('posts_count', 'desc')
            ->get()
            ->mapWithKeys(function ($tag) {
                return [$tag->name => $tag->posts_count];
            });
        return $tags;
    }
}
