<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;

class PostController extends Controller
{
    public function show($serial)
    {
        return view('post.show', compact('serial'));
    }

    public function create()
    {
        return view('post.create');
    }

    public function rank($serial)
    {
        return view('post.rank', compact('serial'));
    }

}
