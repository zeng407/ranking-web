<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;

use App\Http\Resources\PublicPostResource;
use App\Models\Post;
use App\Services\PostService;
use Illuminate\Http\Request;

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
            'public' => true,
            'elements_count_gte' => config('post.post_min_element_count'),
            'any_like' => $request->query('any_like')
        ],[
            'sort_by' => $request->query('sort_by'),
            'sort_dir' => $request->query('sort_dir'),
        ]);

        return PublicPostResource::collection($posts);
    }
}
