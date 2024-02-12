<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Helper\SerialGenerator;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Http\Resources\MyPost\PostRankResource;
use App\Http\Resources\MyPost\PostResource;
use App\Models\Element;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use App\Repositories\Filters\ElementFilter;
use App\Services\ElementService;
use App\Services\RankService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Validation\Rule;
use Arr;
use App\Services\PostService;

class MyPostController extends Controller
{
    const ELEMENTS_PER_PAGE = 50;

    protected ElementService $elementService;
    protected RankService $rankService;
    protected PostService $postService;

    public function __construct(ElementService $elementService, RankService $rankService, PostService $postService)
    {
        $this->elementService = $elementService;
        $this->rankService = $rankService;
        $this->postService = $postService;
        $this->middleware('auth');
    }

    public function index()
    {
        /** @var User $user */
        $user = Auth::user();

        return PostResource::collection($user->posts()->paginate());
    }

    public function show(Post $post)
    {
        /**
         * @see  \App\Policies\PostPolicy::edit()
         */
        $this->authorize('edit', $post);

        return PostResource::make($post);
    }

    public function elements(Request $request, Post $post)
    {
        /**
         * @see \App\Policies\PostPolicy::read()
         */
        $this->authorize('read', $post);

        $input = $request->validate([
            'per_page' => 'integer|max:50',
            'page' => 'integer',
            'filter' => 'json'
        ]);
        \Log::debug($request->input());

        $paginationOptions = $this->parsePaginationOptions($input);
        $condition = $this->parseFilter($input, [
            'post_id' => $post->id
        ], [
            ElementFilter::TITLE_LIKE
        ]);

        $data = $this->elementService->getLists($condition, $paginationOptions);

        return PostElementResource::collection($data);
    }

    public function create(Request $request)
    {
        /**
         * @see \App\Policies\PostPolicy::create()
         */
        $this->authorize('create', Post::class);

        /** @var User $user */
        $user = Auth::user();

        $data = $request->validate([
            'title' => ['required', 'string', 'max:' . config('setting.post_title_size')],
            'description' => ['required', 'string', 'max:' . config('setting.post_description_size')],
            'policy.access_policy' => ['required', Rule::in([PostAccessPolicy::PRIVATE, PostAccessPolicy::PUBLIC])],
        ]);

        $post = $this->postService->create($user, $data);

        return response()->json([
            'serial' => $post->serial
        ]);
    }

    public function update(Request $request, Post $post)
    {
        /**
         * @see \App\Policies\PostPolicy::update()
         */
        $this->authorize('update', $post);

        $data = $request->validate([
            'title' => ['required', 'string', 'max:' . config('setting.post_title_size')],
            'description' => ['required', 'string', 'max:' . config('setting.post_description_size')],
            'policy.access_policy' => [
                'sometimes',
                'required',
                Rule::in([PostAccessPolicy::PUBLIC, PostAccessPolicy::PRIVATE])
            ],
            'policy.password' => 'sometimes|required',
        ]);

        $post->update($data);
        $post->post_policy()->update(data_get($data, 'policy', []));
        return PostResource::make($post);
    }

    public function delete(Request $request, Post $post)
    {
        /**
         * @see \App\Policies\PostPolicy::delete()
         */
        $this->authorize('delete', $post);

        $this->validate($request, [
            'password' => 'required'
        ]);
        if(!password_verify($request->input('password'), Auth::user()->password)){
            return response()->json([
                'message' => 'password is not correct'
            ], 403);
        }

        $this->postService->delete($post);

        return response()->json([
            'message' => 'success'
        ]);
    }


    public function rank(Post $post)
    {
        /**
         * @see \App\Policies\PostPolicy::read()
         */
        $this->authorize('read', $post);
        $reports = $this->rankService->getRankReports($post);

        return PostRankResource::collection($reports);
    }

    protected function parsePaginationOptions($input)
    {
        $paginationOptions = [
            'per_page' => $input['per_page'] ?? self::ELEMENTS_PER_PAGE,
            'page' => $input['per_page'] ?? 1
        ];

        return $paginationOptions;
    }

    protected function parseFilter($input, $preCondition, $only = [])
    {
        if (isset($input['filter'])) {
            $filter = (array)json_decode($input['filter'], true);
            $filter = array_filter($filter);
            if (count($only)) {
                $filter = Arr::only($filter, $only);
            }
            $preCondition += $filter;
        }

        return $preCondition;
    }
}
