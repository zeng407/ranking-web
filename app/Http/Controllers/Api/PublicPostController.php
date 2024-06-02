<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use App\Http\Resources\Comment\CommentResource;
use App\Http\Resources\Game\ChampionResource;
use App\Models\Post;
use App\Models\UserGameResult;
use App\Services\Builders\CommentBuilder;
use App\Services\PostService;
use Illuminate\Http\Request;
use RateLimiter;
use App\Models\Comment;


class PublicPostController extends Controller
{
    protected $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
        $this->middleware('throttle:5,1')->only('createComment');

    }

    public function getPosts(Request $request)
    {
        $request->validate([
            'sort_by' => ['nullable', 'string', 'in:hot,new'],
            'range' => ['nullable', 'string', 'in:all,year,month,week,day'],
            'page' => ['nullable', 'integer', 'min:1'],
            'per_page' => ['nullable', 'integer', 'min:1', 'max:15'],
            'k' => ['nullable', 'string', 'max:255'],
        ]);
        $sort = $this->getSort($request);
        $posts = CacheService::rememberPosts($request, $sort);

        return response()->json($posts);
    }

    public function getChampions(Request $request)
    {
        $games = UserGameResult::with('game', 'champion', 'loser', 'game.post')
            ->whereHas('game.post.post_policy', function ($query) {
                $query->where('access_policy', PostAccessPolicy::PUBLIC);
            })
            ->orderByDesc('id')
            ->cursorPaginate(5);

        return ChampionResource::collection($games);
    }

    public function getComments(Request $request, Post $post)
    {
        $user = $request->user();
        $comments = $this->postService->getComments($post, 10);
        $data = CommentResource::collection($comments)
            ->additional([
                'profile' => [
                    'nickname' => $user? $user->name : config('setting.anonymous_nickname'),
                    'avatar_url' => $user?->avatar_url,
                    'champions' => $this->postService->getUserLastVotes($post, $user, session()->get('anonymous_id', 'unknown')),
                    'is_auth' => $user != null
                ]
            ]);
        return $data;
    }

    public function createComment(Request $request, Post $post)
    {
        $request->validate([
            'content' => ['required', 'string', 'max:' . config('setting.comment_max_length')],
            'anonymous' => 'boolean'
        ]);
        $createComment = function () use ($request, $post) {
            $commentBuilder = (new CommentBuilder)
                ->setPost($post)
                ->setContent($request->input('content'))
                ->setUser($request->user())
                ->setAnonymousId(session()->get('anonymous_id'))
                ->setIp($request->ip())
                ->setLabel(['champions' => $this->postService->getUserLastVotes($post, $request->user(), session()->get('anonymous_id', 'unknown'))])
                ->setAnonymousMode($request->input('anonymous', false));
            return $this->postService->createComment($commentBuilder);
        };

        if ($request->user() == null) {
            /** @var Comment */
            $comment = RateLimiter::attempt(
                'comment:'.($request->ip() ?: 'unknown'),
                $perMinute = 3,
                $createComment
            );
        }else {
            $comment = RateLimiter::attempt(
                'comment:'.$request->user()->id,
                $perMinute = 6,
                $createComment
            );
        }

        if (!$comment) {
            return response()->json(['message' => __('Too many requests')], 429);
        }

        return response()->json([], 201);
    }

    public function report(Request $request, Post $post, Comment $comment)
    {
        $request->validate([
            'reason' => ['nullable', 'string', 'max:' . config('setting.report_max_length')],
        ]);

        $this->postService->reportComment($comment, $request->input('reason'), $request->user(), $request->ip());

        return response()->json([], 201);
    }

    protected function getSort(Request $request)
    {
        $sort = $request->query('sort_by', 'hot');
        $range = $request->query('range', config('setting.home_page_default_range'));

        if ($sort === 'hot') {
            $sort = 'hot_' . $range;
        }

        return $sort;
    }

}
