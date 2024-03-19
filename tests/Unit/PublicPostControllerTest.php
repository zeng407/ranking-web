<?php

use App\Http\Controllers\Api\PublicPostController;
use App\Services\PostService;
use Illuminate\Http\Request;
use Tests\TestCase;
use App\Models\Post;
use App\Models\User;
use App\Models\Comment;

class PublicPostControllerTest extends TestCase
{

    public function test_get_comments()
    {
        $post = Post::factory()->create();
        $post->comments()->create(Comment::factory()->make()->toArray());
        $res = $this->get(route('api.public-post.comment.index', $post->serial));
        $res->assertOk();

        $expected = $post->comments()->paginate()->pluck('id')->toArray();
        $actual = collect(json_decode($res->content())->data)->pluck('id')->toArray();

        $this->assertEquals($expected, $actual);
    }

    public function test_create_comment()
    {
        $post = Post::factory()->create();
        $user = User::factory()->create();
        $content = 'Test comment content';

        $this->post(route('api.public-post.comment.create', $post->serial), [
            'content' => $content
        ])->assertCreated();

        $this->assertDatabaseHas('comments', [
            'content' => $content,
        ]);

        $this->assertDatabaseHas('post_comments', [
            'post_id' => $post->id,
        ]);
    }

    public function test_report_comment()
    {
        
        $user = User::factory()->create();
        $comment = Comment::factory()->has(Post::factory(), 'posts')->create();
        $reason = 'Test report reason';
        $post = $comment->getPost();
        $this->be($user);

        $response = $this->post(route('api.public-post.comment.report', [$post->serial, $comment->id]), [
            'reason' => $reason
        ]);

        $response->assertStatus(201);

        $this->assertDatabaseHas('reported_comments', [
            'comment_id' => $comment->id,
            'reason' => $reason,
            'reporter_id' => $user->id,
        ]);
    }
}