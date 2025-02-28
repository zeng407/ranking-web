<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Http\Resources\MyPost\PostRankResource;
use App\Http\Resources\MyPost\MyPostResource;
use App\Models\Post;
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
    const ELEMENTS_PER_PAGE = 100;

    protected ElementService $elementService;
    protected RankService $rankService;
    protected PostService $postService;

    public function __construct(ElementService $elementService, RankService $rankService, PostService $postService)
    {
        $this->elementService = $elementService;
        $this->rankService = $rankService;
        $this->postService = $postService;
    }

    public function index()
    {
        /** @var User $user */
        $user = Auth::user();
        $post = $user->posts()
            ->with(['post_policy','tags'])
            ->paginate();

        return MyPostResource::collection($post);
    }

    public function show(Post $post)
    {
        /**
         * @see  \App\Policies\PostPolicy::edit()
         */
        $this->authorize('edit', $post);

        return MyPostResource::make($post);
    }

    public function elements(Request $request, Post $post)
    {
        /**
         * @see \App\Policies\PostPolicy::read()
         */
        $this->authorize('read', $post);

        $input = $request->validate([
            'per_page' => ['integer', 'max:'.self::ELEMENTS_PER_PAGE],
            'page' => 'integer',
            'filter' => 'array',
            'sort_by' => 'nullable|string',
            'sort_dir' => 'nullable|string|in:asc,desc'
        ]);

        $paginationOptions = $this->parsePaginationOptions($input);
        $condition = $this->parseFilter($input, [
            'post_id' => $post->id
        ], [
            ElementFilter::TITLE_LIKE
        ]);
        $sort = $this->parseSorter($input, [
            'id' => 'desc'
        ]);
        $with = [
            'rank_reports' => fn($query) => $query->where('post_id', $post->id)
        ];

        $data = $this->elementService->getList($condition, $sort, $paginationOptions, $with);

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
            'policy.access_policy' => ['required', Rule::in([PostAccessPolicy::PRIVATE , PostAccessPolicy::PUBLIC, PostAccessPolicy::PASSWORD])],
            'policy.password' => 'required_if:policy.access_policy,' . PostAccessPolicy::PASSWORD,
        ]);

        if($data['policy']['access_policy'] === PostAccessPolicy::PASSWORD) {
            $data['policy']['password'] = hash('sha256', $data['policy']['password']);
        }

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
                Rule::in([PostAccessPolicy::PUBLIC , PostAccessPolicy::PRIVATE, PostAccessPolicy::PASSWORD])
            ],
            'policy.password' => ['sometimes', 'string', 'max:255'],
            'tags' => ['sometimes', 'array', 'between:0,' . config('setting.post_max_tags')],
            'tags.*' => ['sometimes', 'string', 'max:' . config('setting.tag_name_size')],
        ]);

        $data = $this->validatePostPassword($request, $post, $data);

        $this->postService->update($post, $data);
        $this->postService->syncTags($post, data_get($data, 'tags', []));
        return MyPostResource::make($post->refresh());
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
        if (!password_verify($request->input('password'), Auth::user()->password)) {
            return response()->json([
                'message' => 'password is not correct'
            ], 403);
        }

        $this->postService->delete($post);

        return response()->json([
            'message' => 'success'
        ]);
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
            $filter = array_filter($input['filter']);
            if (count($only)) {
                $filter = Arr::only($filter, $only);
            }
            $preCondition += $filter;
        }

        return $preCondition;
    }

    protected function parseSorter($input, $default, $only = [])
    {
        if(isset($input['sort_by'])) {
            $sortBy = count($only) ? Arr::only($input['sort_by'], $only) : $input['sort_by'];
            $sorter = [
                'sort_by' => $sortBy,
                'sort_dir' => $input['sort_dir'] ?? 'asc'
            ];
        }else{
            $sorter = $default;
        }

        return $sorter;
    }

    protected function validatePostPassword(Request $request, $post, $data)
    {
        $policy = $request->input('policy.access_policy');
        $password = $request->input('policy.password');
        if($policy === PostAccessPolicy::PASSWORD
            && $post->post_policy->password == null){
            $request->validate([
                'policy.password' => ['required', 'string', 'max:255']
            ]);
        }
        if($policy === PostAccessPolicy::PASSWORD) {
            if(!empty($password)){
                $data['policy']['password'] = hash('sha256', $password);
            }else{
                unset($data['policy']['password']);
            }
        }else{
            $data['policy']['password'] = null;
        }

        return $data;
    }
}
