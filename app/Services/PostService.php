<?php


namespace App\Services;


use App\Models\Post;
use App\Models\Tag;
use App\Models\UserGameResult;
use App\Repositories\PostRepository;
use App\Models\User;
use App\Helper\SerialGenerator;
use App\Events\PostCreated;
use App\Events\PostDeleted;
use App\Models\Comment;
use App\Services\Builders\CommentBuilder;
use App\Services\Traits\HasRepository;
use Ramsey\Uuid\Uuid;

class PostService
{
    use HasRepository;
    
    protected $repo;

    public function __construct(PostRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getPost($serial): ?Post
    {
        return Post::where('serial', $serial)->first();
    }

    public function create(User $user, array $data)
    {
        /** @var Post $post */
        $post = $user->posts()->create([
            'serial' => SerialGenerator::genPostSerial()
        ] + $data);

        $post->post_policy()->updateOrCreate(data_get($data, 'policy', []));

        $post->imgur_album()->create([
            'title' => $post->title,
            'description' => $post->description
        ]);

        event(new PostCreated($post));

        return $post;
    }

    public function delete(Post $post)
    {
        $post->tags()->detach();
        $post->delete();
        event(new PostDeleted($post));
    }

    public function syncTags(Post $post, array $tags)
    {
        $post->tags()->detach();
        foreach ($tags as $tag) {
            if (is_string($tag) && !empty($tag)) {
                $tagModel = Tag::firstOrCreate(['name' => $tag]);
                $post->tags()->attach($tagModel->id);
            }
        }
    }

    public function getComments(Post $post, int $number = 10)
    {
        return $post->comments()->latest()->paginate($number);
    }

    public function createComment(CommentBuilder $commentBuilder): Comment
    {
        return $commentBuilder->build();
    }

    public function getUserLastVotes(Post $post, ?User $user, string $anonymousId, int $number = 1): array
    {
        $champions = UserGameResult::where(function ($query) use ($user, $anonymousId) {
            if ($user instanceof User) {
                $query->where('user_id', $user->id);
            } else {
                $query->where('anonymous_id', $anonymousId);
            }
        })
            ->whereHas('game.post', function ($query) use ($post) {
                $query->where('id', $post->id);
            })
            ->orderBy('id', 'desc')
            ->limit($number)
            ->get()
            ->pluck('champion_name')
            ->toArray();

        return $champions;
    }

    public function reportComment(Comment $comment, ?string $reason, ?User $reporter = null, ?string $reporterIp = null)
    {
        $comment->abuse_reports()->create([
            'comment_content' => $comment->content,
            'reason' => $reason,
            'reporter_id' => $reporter?->id,
            'reporter_ip' => $reporterIp
        ]);
    }
}
