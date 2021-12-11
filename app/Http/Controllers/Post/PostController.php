<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;

class PostController extends Controller
{
    public function index()
    {
        return view('post.index');
    }

    public function edit($serial)
    {
        return view('post.edit', compact('serial'));
    }

}
