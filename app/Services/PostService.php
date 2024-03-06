<?php


namespace App\Services;


use App\Models\Post;
use App\Models\Tag;
use App\Repositories\PostRepository;
use App\Models\User;
use App\Helper\SerialGenerator;
use App\Events\PostCreated;
use App\Events\PostDeleted;

class PostService
{
    protected $repo;

    public function __construct(PostRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getPost($serial): ?Post
    {
        return Post::where('serial', $serial)->first();
    }

    public function getLists(array $conditions, array $sorter = [] , array $paginationOptions = [])
    {
        //trim null or empty
        $conditions = array_filter($conditions, fn($value) => !is_null($value) && $value !== '');
        $sorter = array_filter($sorter, fn($value) => !is_null($value) && $value !== '');

        $query = $this->repo->filter($conditions);

        if($sortBy = data_get($sorter, 'sort_by')){
            $dir = data_get($sorter, 'sort_dir') === 'desc' ? 'desc' : 'asc';
            if($sortBy === 'hot'){
                $sortRange = data_get($sorter,'sort_range', 'month');
                $sortBy = 'hot_'.$sortRange;
            }
            $query = $this->repo->sorter($query, $sortBy, $dir);
            logger($sortBy);
        }

        $perPage = data_get($paginationOptions, 'per_page', 15);

        return $query->paginate($perPage);
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
            if (is_string($tag) && !empty($tag) ) {
                $tagModel = Tag::firstOrCreate(['name' => $tag]);
                $post->tags()->attach($tagModel->id);
            }
        }
    }
}
