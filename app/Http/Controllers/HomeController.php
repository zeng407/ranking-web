<?php

namespace App\Http\Controllers;

use App\Http\Resources\PublicPostResource;
use App\Repositories\Filters\PostFilter;
use Illuminate\Http\Request;
use App\Services\TagService;
use App\Services\PostService;

class HomeController extends Controller
{
    protected $tagService;
    protected $postService;

    public function __construct(TagService $tagService, PostService $postService)
    {
        $this->tagService = $tagService;
        $this->postService = $postService;
    }

    public function index(Request $request)
    {
        $sort = $request->query('sort_by', 'hot');
        $range = $request->query('range', 'month');
        $tags = $this->tagService->get(null);
        $posts = $this->postService->getLists([
            PostFilter::PUBLIC => true,
            PostFilter::ELEMENTS_COUNT_GTE => config('setting.post_min_element_count'),
            PostFilter::KEYWORD_LIKE => $request->query('k')
        ],[
            'sort_by' => $sort,
            'sort_range' => $range,
            'sort_dir' => $request->query('sort_dir'),
        ]);

        foreach( $posts as $key => $post ) {
            $posts[$key] = PublicPostResource::make($post)->toArray($request);
        }
    
        return view('home', [
            'sort' => $sort,
            'range'=> $range,
            'tags' => $tags,
            'posts' => $posts
        ]);
    }
}
