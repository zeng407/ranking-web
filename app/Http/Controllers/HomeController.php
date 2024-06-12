<?php

namespace App\Http\Controllers;

use App\Helper\CacheService;
use Illuminate\Http\Request;
use App\Services\PostService;

class HomeController extends Controller
{
    protected $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
    }

    public function index(Request $request)
    {
        $request->validate([
            'sort_by' => ['nullable', 'string', 'in:hot,new'],
            'range' => ['nullable', 'string', 'in:all,year,month,week,day'],
            'k' => ['nullable', 'string', 'max:255'],
        ]);
        $sort = $this->getSort($request);
        $posts = $this->getPosts($request, $sort);

        return view('home', [
            'sort' => $request->query('sort_by', 'hot'),
            'range' => $request->query('range', config('setting.home_page_default_range')),
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

    protected function getPosts(Request $request, $sort)
    {
        return CacheService::rememberPosts($request, $sort);
    }

    public function lang(Request $request, $locale)
    {
        session()->put('locale', $locale);
        app()->setLocale($locale);
        return $this->index($request);
    }
}
