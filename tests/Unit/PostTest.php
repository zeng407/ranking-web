<?php

namespace Tests\Unit;

use App\Enums\ElementType;
use App\Enums\PostAccessPolicy;
use App\Models\Element;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use Google\Service\YouTube\AccessPolicy;
use Illuminate\Foundation\Testing\DatabaseMigrations;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Storage;

use Tests\TestCase;
use Illuminate\Support\Facades\Http;

class PostTest extends TestCase
{
    public function test_user_post()
    {
        /** @var User $user */
        $user = User::factory()->has(Post::factory()->count(3))->create();

        $post = Post::first();
        $this->assertTrue($user->posts()->first()->is($post));
        $this->assertTrue($post->user->is($user));
    }

    public function test_post_elements()
    {
        /** @var Post $post */
        $post = Post::factory()->create();

        /** @var Element $element */
        $element = $post->elements()->create(Element::factory()->create()->toArray());
        $this->assertTrue($post->elements()->first()->is($element));
        $this->assertTrue($element->posts()->first()->is($post));

        /** @var Element $element */
        $element = $post->elements()->create(Element::factory()->video()->create()->toArray());
        $this->assertTrue($element->posts()->first()->is($post));

        /** @var Element $element */
        $element = $post->elements()->create(Element::factory()->image()->create()->toArray());
        $this->assertTrue($element->posts()->first()->is($post));
    }

    public function test_post_api()
    {
        /** @var User $user */
        $user = User::factory()->create();

        $this->be($user);

        $res = $this->post(route('api.post.create'), [
            'title' => 'title',
            'description' => 'description',
            'policy' => [
                'access_policy' => PostAccessPolicy::PUBLIC
            ]
        ]);

        $serial = $res->json('serial');

        $this->assertTrue(
            Post::where('serial', $serial)->first()->is(
                $user->posts()->first()
            )
        );
    }

    public function test_post_update()
    {
        /** @var User $user */
        $user = User::factory()->has(
            Post::factory()->has(
                PostPolicy::factory()->public(), 'post_policy'
            )
        )->create();

        $this->be($user);

        $postSerial = $user->posts()->first()->serial;
        $body = [
            'title' => 'rand title',
            'description' => 'rand description',
            'policy' => [
                'access_policy' => PostAccessPolicy::PUBLIC,
            ]
        ];
        $this->put(route('api.post.update', $postSerial), $body);

        /** @var Post $post */
        $post = $user->posts()->first();
        $this->assertEquals($post->serial, $postSerial);
        $this->assertEquals($post->title, 'rand title');
        $this->assertEquals($post->description, 'rand description');
        $this->assertEquals($post->post_policy->access_policy, PostAccessPolicy::PUBLIC);
        $this->assertEquals($post->post_policy->password, null);
    }

    public function test_create_image_element()
    {
        /** @var User $user */
        $user = User::factory()->has(Post::factory())->create();

        $this->be($user);

        Http::fake([
            'image' => Http::response([
                'data' => [
                    'id' => 'anyid',
                    'deletehash' => 'anydeletehash',
                    'title' => 'anytitle',
                    'description' => 'anydescription',
                    'link' => 'anylink',
                ],
                'success' => true,
            ], 200),
        ]);

        /** @var Post $post */
        $post = $user->posts()->first();
        $post->imgur_album()->create([
            'album_id' => 'anyid',
            'deletehash' => 'anydeletehash',
            'title' => 'anytitle',
            'description' => 'anydescription',
        ]);

        $file = UploadedFile::fake()->image('random_image.jpg');
        $data = [
            'post_serial' => $post->serial,
            'file' => $file,
        ];

        $res = $this->post(route('api.element.create-image'), $data);

        $path = $res->json('path');
        Storage::disk()->assertExists($path);
        \Log::debug($res->content());
        $this->assertEquals($post->elements()->first()->id, $res->json('data.id'));
        $this->assertEquals($res->json('data.type'), ElementType::IMAGE);
    }

    public function test_create_video_element()
    {
        /** @var User $user */
        $user = User::factory()->has(
            Post::factory()->has(
                PostPolicy::factory(),'post_policy'
            )
        )->create();

        $this->be($user);

        /** @var Post $post */
        $post = $user->posts()->first();

        $data = [
            'post_serial' => $post->serial,
            'url' => 'https://www.youtube.com/watch?v=dQw4w9WgXcQ',
            'video_start_second' => '34',
            'video_end_second' => '78'
        ];

        $res = $this->post(route('api.element.create-video-youtube'), $data);
        $res->assertCreated();
        $this->assertEquals($post->elements()->first()->id, $res->json('data.id'));
        $this->assertEquals($res->json('data.type'), ElementType::VIDEO);
        $this->assertEquals($res->json('data.video_start_second'), '34');
        $this->assertEquals($res->json('data.video_end_second'), '78');
        $this->assertEquals($res->json('data.source_url'), 'https://www.youtube.com/watch?v=dQw4w9WgXcQ');
    }
}
