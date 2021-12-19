<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Http\Resources\PostElementResource;
use App\Http\Resources\PostResource;
use App\Models\Element;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use App\Repositories\Filters\ElementFilter;
use App\Services\ElementService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Validation\Rule;
use Arr;

class PostController extends Controller
{
    const ELEMENTS_PER_PAGE = 50;

    protected $elementService;

    public function __construct(ElementService $elementService)
    {
        $this->elementService = $elementService;
        $this->middleware('auth');
    }

    public function index()
    {
        /** @var User $user */
        $user = Auth::user();

        return PostResource::collection($user->posts()->paginate());
    }

    public function show($serial)
    {
        /** @var User $user */
        $user = Auth::user();
        $post = $user->posts()->where('serial', $serial)->firstOrFail();

        return PostResource::make($post);
    }

    public function elements(Request $request, $serial)
    {
//        \Log::debug('query',$request->query());
        $input = $request->validate([
            'per_page' => 'integer|max:50',
            'page' => 'integer',
            'filter' => 'json'
        ]);
//        \Log::debug('input',$input);
        /** @var User $user */
        $user = Auth::user();

        /** @var Post $post */
        $post = $user->posts()->where('serial', $serial)->firstOrFail();

        $condition = [
            'post_id' => $post->id
        ];
        $paginationOptions = [
            'per_page' => $input['per_page'] ?? self::ELEMENTS_PER_PAGE,
            'page' => $input['per_page'] ?? 1
        ];
        if(isset($input['filter'])) {
            $filter = (array) json_decode($input['filter'], true);
            $filter = array_filter($filter);
            $filter = Arr::only($filter, [ElementFilter::TITLE_LIKE]);
//            \Log::debug('filter', $filter);
            $condition += $filter;
        }
        $data = $this->elementService->getLists($condition, $paginationOptions);

        return PostElementResource::collection($data);
    }

    public function create(Request $request)
    {
        /** @var User $user */
        $user = Auth::user();

        $data = $request->validate([
            'title' => 'required',
            'description' => 'required',
            'policy.access_policy' => ['required', Rule::in([PostAccessPolicy::PRIVATE, PostAccessPolicy::PUBLIC])],
        ]);

        /** @var Post $post */
        $post = $user->posts()->create([
                'serial' => bin2hex(random_bytes(3)) // todo give a unique key
            ] + $data);

        $post->post_policy()->updateOrCreate(data_get($data, 'policy', []));

        return response()->json([
            'serial' => $post->serial
        ]);
    }

    public function update(Request $request, $serial)
    {
        $data = $request->validate([
            'title' => 'sometimes|required',
            'description' => 'sometimes|required',
            'policy.access_policy' => ['sometimes', 'required',
                Rule::in([PostAccessPolicy::PUBLIC, PostAccessPolicy::PRIVATE])],
            'policy.password' => 'sometimes|required',
        ]);
        \Log::debug($data);
        /** @var Post $post */
        $post = Post::where('user_id', Auth::id())->where('serial', $serial)->firstOrFail();

        $post->update($data);
        $post->post_policy()->update(data_get($data, 'policy', []));
        return PostResource::make($post);
    }
}
