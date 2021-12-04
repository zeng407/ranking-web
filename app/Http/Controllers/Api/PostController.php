<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\PostResource;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class PostController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth');
    }

    public function index()
    {
        /** @var User $user */
        $user = Auth::user();

        return PostResource::collection($user->posts()->paginate());
    }

    public function create()
    {
        /** @var User $user */
        $user = Auth::user();

        /** @var Post $post */
        $post = $user->posts()->create([
           'serial' => bin2hex(random_bytes(3)) // todo give a unique key
        ]);

        return $post;
    }

    public function update(Request $request, $serial)
    {
        $data = $request->validate([
            'title' => 'sometimes|required',
            'description' => 'sometimes|required',
            'policy.access_policy' => 'sometimes|required',
            'policy.password' => 'sometimes|required',
        ]);

        /** @var Post $post */
        $post = Post::where('user_id', Auth::id())->where('serial', $serial)->firstOrFail();

        $post->update($data);
        $post->post_policy()->updateOrCreate(data_get($data, 'policy', []));

        return $post;
    }
}
