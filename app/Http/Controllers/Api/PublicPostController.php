<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\Comment\CommentResource;
use App\Http\Resources\PublicPostResource;
use App\Models\Post;
use App\Services\Builders\CommentBuilder;
use App\Services\PostService;
use Illuminate\Http\Request;
use App\Repositories\Filters\PostFilter;
use Illuminate\Pagination\LengthAwarePaginator;
use RateLimiter;
use App\Models\Comment;


class PublicPostController extends Controller
{
    protected $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
    }

    public function index(Request $request)
    {
        $posts = $this->postService->getLists([
            PostFilter::PUBLIC => true,
            PostFilter::ELEMENTS_COUNT_GTE => config('setting.post_min_element_count'),
            PostFilter::KEYWORD_LIKE => $request->query('k')
        ], [
            'sort_by' => $request->query('sort_by'),
            'sort_dir' => $request->query('sort_dir'),
        ]);

        return PublicPostResource::collection($posts);
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
                    'champions' => $this->postService->getUserLastVotes($post, $user, session()->get('anonymous_id', 'unknown'))
                ]
            ]);
        return $data;
    }

    public function createComment(Request $request, Post $post)
    {
        $request->validate([
            'content' => ['required', 'string', 'max:' . config('setting.comment_max_length')],
            'anonymount_mode' => 'boolean'
        ]);
        $createComment = function () use ($request, $post) {
            $commentBuilder = (new CommentBuilder)
                ->setPost($post)
                ->setContent($request->input('content'))
                ->setUser($request->user())
                ->setAnonymousId(session()->get('anonymous_id'))
                ->setIp($request->ip())
                ->setLabel(['champions' => $this->postService->getUserLastVotes($post, $request->user(), session()->get('anonymous_id', 'unknown'))])
                ->setAnonymousMode($request->input('anonymous_mode', false));
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
}
