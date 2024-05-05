<?php

namespace App\Http\Controllers;

use App\Helper\CacheService;
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
        $sort = $this->getSort($request);
        $tags = $this->getHotTags();
        $posts = $this->getPosts($request, $sort);
    
        return view('home', [
            'sort' => $request->query('sort_by', 'hot'),
            'range'=> $request->query('range', config('setting.home_page_default_range')),
            'tags' => $tags,
            'posts' => $posts
        ]);
    }

    protected function getSort(Request $request)
    {
        $sort = $request->query('sort_by', 'hot');
        $range = $request->query('range', config('setting.home_page_default_range'));

        if ($sort === 'hot') {
            $sort = 'hot_' . $range;
        }

        return $sort;
    }

    protected function getHotTags()
    {
        return CacheService::rememberHotTags();
    }

    protected function getPosts(Request $request, $sort)
    {
        return CacheService::rememberPosts($request, $sort);
    }

    public function lang(Request $request , $locale)
    {
        session()->put('locale', $locale);
        app()->setLocale($locale);
        return $this->index($request);
    }
}
