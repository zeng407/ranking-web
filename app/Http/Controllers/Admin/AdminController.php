<?php

namespace App\Http\Controllers\Admin;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Models\Tag;
use App\Services\PostService;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;
use App\Models\Element;

class AdminController extends Controller
{
    protected PostService $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
    }
    
    public function dashboard()
    {
        return view('admin.dashboard');
    }

    public function indexPost()
    {
        $posts = Post::orderByDesc('id')->paginate(10);
        return view('admin.post.index', compact('posts'));
    }

    public function showPost($postId)
    {
        $post = Post::findOrFail($postId);
        return view('admin.post.show', compact('post'));
    }

    public function updatePost(Request $request, $postId)
    {
        $data = $request->validate([
            'title' => ['required', 'string', 'max:' . config('setting.post_title_size')],
            'description' => ['required', 'string', 'max:' . config('setting.post_description_size')],
            'policy.access_policy' => [
                'sometimes',
                'required',
                Rule::in([PostAccessPolicy::PUBLIC , PostAccessPolicy::PRIVATE ])
            ],
            'policy.password' => 'sometimes|required',
            'tags' => ['sometimes', 'array', 'between:0,' . config('setting.post_max_tags')],
            'tags.*' => ['sometimes','nullable', 'string', 'max:'.config('setting.tag_name_size')],
        ]);

        $post = Post::findOrFail($postId);
        $this->postService->update($post, $data);
        $this->postService->syncTags($post, data_get($data, 'tags', []));
        return redirect()->back()->with('success', 'Post updated successfully!');
    }

    public function deletePost($postId)
    {
        $post = Post::findOrFail($postId);
        $post->delete();
        return redirect()->route('admin.post.index')->with('success', 'Post deleted successfully!');
    }

    public function indexElement($postId)
    {
        $elements = Element::where('post_id', $postId)->orderByDesc('id')->paginate(10);
        return view('admin.element.index', compact('elements'));
    }

    public function updateElement(Request $request, $postId, $elementId)
    {
        $data = $request->validate([
            'title' => ['sometimes', 'string', 'max:' . config('setting.element_title_size')],
        ]);

        $element = Element::findOrFail($elementId);
        $element->update($data);
        return redirect()->back()->with('success', 'Element updated successfully!');
    }

    public function deleteElemenet($postId, $elementId)
    {
        $element = Element::findOrFail($elementId);
        $element->delete();
        return redirect()->back()->with('success', 'Element deleted successfully!');
    }

}
