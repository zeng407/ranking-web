<?php

namespace App\Services;

use App\Enums\PostAccessPolicy;
use App\Helper\CacheService;
use App\Models\PublicPost;
use App\Repositories\Filters\PostFilter;
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
        $posts = app(PublicPostService::class)->getList(
            [],[
            'sort_by' => 'hot_week',
            'sort_dir' => 'week',
        ], [
            'per_page' => $limit
        ]);

        $tags = [];
        foreach ($posts as $post) {
            foreach (json_decode($post->tags,true) as $tag) {
                if(!in_array($tag, $tags)) {
                    $tags[] = $tag;
                }
            }
        }
        $tags = collect($tags)
            ->mapWithKeys(function ($tag) {

                $count = PublicPost::where('tags', 'like', '%' . $tag . '%')
                    ->orWhere('title', 'like', '%' . $tag . '%')
                    ->orWhere('description', 'like', '%' . $tag . '%')
                    ->count();
                return [$tag => $count];
            });
        return $tags;
    }
}
