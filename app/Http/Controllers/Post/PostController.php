<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Policies\PostPolicy;
use Illuminate\Http\Request;

class PostController extends Controller
{
    
    public function index()
    {
        return view('account.post.index');
    }

    public function edit(Post $post)
    {
        /** @see PostPolicy::update() */
        $this->authorize('update', $post);

        $config = [
            'post_max_element_count' => config('setting.post_max_element_count'),
            'post_title_size' => config('setting.post_title_size'),
            'post_description_size' => config('setting.post_description_size'),
            'element_title_size' => config('setting.element_title_size'),
        ];

        return view('account.post.edit', [
            'serial' => $post->serial,
            'post' => $post,
            'config' => $config,
        ]);
    }

}
