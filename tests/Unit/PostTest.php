<?php

namespace Tests\Unit;

use App\Enums\ElementType;
use App\Enums\PostAccessPolicy;
use App\Models\Element;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use Faker\Factory;
use Illuminate\Foundation\Testing\DatabaseMigrations;
use Illuminate\Http\Testing\File;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Storage;
use Illuminate\Support\Str;
use Tests\TestCase;

class PostTest extends TestCase
{
    use DatabaseMigrations;

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

        $res = $this->post(route('api.post.create'));

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
        $user = User::factory()->has(Post::factory())->create();

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

        /** @var Post $post */
        $post = $user->posts()->first();

        $file = UploadedFile::fake()->image('random_image.jpg');
        $data = [
            'post_serial' => $post->serial,
            'file' => $file,
        ];

        $res = $this->post(route('api.element.create-image'), $data);

        $path = $res->json('path');
        Storage::disk()->assertExists($path);
        $this->assertEquals($post->elements()->first()->id, $res->json('id'));
        $this->assertEquals($res->json('type'), ElementType::IMAGE);
    }

    public function test_create_video_element()
    {
        /** @var User $user */
        $user = User::factory()->has(Post::factory())->create();

        $this->be($user);

        /** @var Post $post */
        $post = $user->posts()->first();
        $element = Element::factory()->video()->make();
        $data = [
            'post_serial' => $post->serial,
            'url' => $element->source_url,
            'video_start_second' => $element->video_start_second,
            'video_end_second' => $element->video_end_second
        ];

        $res = $this->post(route('api.element.create-video'), $data);
        $this->assertEquals($post->elements()->first()->id, $res->json('id'));
        $this->assertEquals($res->json('type'), ElementType::VIDEO);
        $this->assertEquals($res->json('video_start_second'), $element->video_start_second);
        $this->assertEquals($res->json('video_end_second'), $element->video_end_second);
        $this->assertEquals($res->json('url'), $element->url);
    }
}
