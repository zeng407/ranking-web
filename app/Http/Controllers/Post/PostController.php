<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Policies\PostPolicy;
use Illuminate\Http\Request;

class PostController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth');
    }

    public function index()
    {
        return view('account.post.index');
    }

    public function edit(Post $post)
    {
        /** @see PostPolicy::update() */
        $this->authorize('update', $post);

        return view('account.post.edit', [
            'serial' => $post->serial
        ]);
    }

}
