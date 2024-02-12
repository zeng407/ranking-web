<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\PublicPostResource;
use App\Services\PostService;
use Illuminate\Http\Request;
use App\Repositories\Filters\PostFilter;

class PublicPostController extends Controller
{
    protected $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
    }

    public function index(Request $request)
    {
        $posts = $this->postService->getLists([
            PostFilter::PUBLIC => true,
            PostFilter::ELEMENTS_COUNT_GTE => config('post.post_min_element_count'),
            PostFilter::ANY_LIKE => $request->query('any_like')
        ],[
            'sort_by' => $request->query('sort_by'),
            'sort_dir' => $request->query('sort_dir'),
        ]);

        return PublicPostResource::collection($posts);
    }
}
