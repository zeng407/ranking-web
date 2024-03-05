<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Models\Tag;
use Illuminate\Http\Request;

class TagController extends Controller
{
    public function index(Request $request)
    {
        $prompt = $request->input("prompt");

        $query = Tag::join('post_tags', 'tags.id', '=', 'post_tags.tag_id')
            ->whereHas('posts.post_policy', function ($query) {
                $query->where('access_policy', PostAccessPolicy::PUBLIC);
            })
            ->groupBy('tags.id')
            ->orderByRaw('count(tags.id) desc')
            ->select('tags.name', \DB::raw('count(tags.id) as count'))
            ->limit(5);

        if (!is_null($prompt) && $prompt !== '') {
            $tags = $query->where('name', 'like', "%$prompt%");
        }

        $tags = $query->get();

        foreach ($tags as $tag) {
            // hide the count for now
            $tag->count = 0;
            // $tag->count = max(((int) ($tag->count / 10)) * 10, 10);
        }

        return response()->json($tags);
    }
}
