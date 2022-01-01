<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Post;
use Illuminate\Http\Request;

class PostController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth');
    }

    public function index()
    {
        return view('post.index');
    }

    public function edit($serial)
    {
        $p = Post::where('serial', $serial)->firstOrFail();
        $this->authorize('update', $p);

        return view('post.edit', compact('serial'));
    }

}
