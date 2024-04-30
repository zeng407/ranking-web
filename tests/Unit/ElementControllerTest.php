<?php

use Illuminate\Http\UploadedFile;
use Tests\TestCase;
use App\Models\Post;
use App\Models\User;
use App\Models\Comment;
use Tests\TestHelper;

class ElementControllerTest extends TestCase
{

    use TestHelper;

    public function testUpdateElementTitle()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        // update title but without post_serial
        $res = $this->put(route('api.element.update', $element->id), [
            'title' => 'new title',
        ], ['Accept' => 'application/json']);
        $res->assertStatus(422);

        // update title
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'title' => 'new title',
        ], ['Accept' => 'application/json']);
        $res->assertStatus(200);
        $res->assertJsonFragment(['title' => 'new title']);
        $this->assertDatabaseHas('elements', ['title' => 'new title']);
    }

    public function testUpdateElementUnknown()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        // update unknown url
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => 'http://example.com',
        ], ['Accept' => 'application/json']);
        $res->assertStatus(422);

    }

    public function testUpdateElementImage()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        // update image url
        $urls = [
            'https://example.com/image.jpg',
            'https://upload.wikimedia.org/wikipedia/en/a/a9/Example.jpg?20240301091138',
            'https://upload.wikimedia.org/wikipedia/commons/7/70/Example.png',
            'https://www.easygifanimator.net/images/samples/eglite.gif',
            'https://i.imgur.com/8nLFCVP.png'
        ];
        foreach ($urls as $url) {
            $res = $this->put(route('api.element.update', $element->id), [
                'post_serial' => $post->serial,
                'url' => $url,
            ], ['Accept' => 'application/json']);
            $res->assertStatus(200);
            $this->assertDatabaseHas('elements', ['source_url' => $url]);
        }

        // update element by uploading image from local
        $res = $this->post(route('api.element.upload', $element->id), [
            'post_serial' => $post->serial,
            'file' => $file = UploadedFile::fake()->image('random_image.jpg')
        ], ['Accept' => 'application/json']);
        $res->assertStatus(200);
        $res->assertJsonStructure(['url', 'path_id']);
        $url = $res->json('url');
        //assert cache exists

        $cache = Cache::get($res->json('path_id'));
        $this->assertEquals([
                'path' => str_replace('/storage/','',$url),
                'is_image' => true,
            ], $cache);

        // mock http request for imgur
        Http::fake([
            'https://api.imgur.com/3/image' => Http::response([
                'data' => [
                    'id' => '8nLFCVP',
                    'title' => 'title',
                    'link' => 'https://i.imgur.com/8nLFCVP.png',
                    'description' => 'description',
                    'type' => 'image/png',
                ],
                'success' => true,
                'status' => 200,
            ]),
            'https://api.imgur.com/3/image/8nLFCVP' => Http::response([
                'data' => [
                    'id' => '8nLFCVP',
                    'title' => 'title',
                    'link' => 'https://i.imgur.com/8nLFCVP.png',
                    'description' => 'description',
                    'type' => 'image/png',
                ],
                'success' => true,
                'status' => 200,
            ]),
            'https://api.imgur.com/3/image/pPEHoApUaxS220Q' => Http::response([
                'data' => [
                    'id' => '8nLFCVP',
                    'title' => 'title',
                    'link' => 'https://i.imgur.com/8nLFCVP.png',
                    'description' => 'description',
                    'type' => 'image/png',
                ],
                'success' => true,
                'status' => 200,
            ]),
        ]);


        // update imgur url
        $urls = [
            'https://imgur.com/gallery/8nLFCVP',
            'https://imgur.com/gallery/yGhDZJ5',
            'https://imgur.com/t/birds/W7Mod3p'
        ];
        foreach ($urls as $url) {
            $res = $this->put(route('api.element.update', $element->id), [
                'post_serial' => $post->serial,
                'url' => $url,
            ], ['Accept' => 'application/json']);
            $res->assertStatus(200);
            $url = $res->json('data')['source_url'];
            $this->assertDatabaseHas('elements', ['source_url' => $url]);
        }
        
    }

    public function testUpdateElementVideo()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);
        // update video url
        $url = 'http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);
        $res->assertStatus(200);
        $this->assertDatabaseHas('elements', ['source_url' => $url]);
    }

    public function testUpdateElementYoutube()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        //update youtube url
        $urls = [
            'https://www.youtube.com/watch?v=0SyTa7D62zQ',
            'https://www.youtube.com/shorts/-YGfTA0qFf0',
        ];
        foreach ($urls as $url) {
            $res = $this->put(route('api.element.update', $element->id), [
                'post_serial' => $post->serial,
                'url' => $url,
            ], ['Accept' => 'application/json']);
            $res->assertStatus(200);
            $this->assertDatabaseHas('elements', ['source_url' => $url]);
        }

        // update youtube clip url failed
        $url = 'https://www.youtube.com/clip/Ugkx4Pim6GGBgjMDm2nUtfYyfR-uenidVxuF';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);
        $res->assertStatus(422);
        $this->assertDatabaseMissing('elements', ['source_url' => $url]);

        // update embed code
        $url = '<iframe width="100%" height="270" src="https://www.youtube.com/embed/0SyTa7D62zQ?si=2Zt5VVfMc-bcESVN&amp;clip=UgkxTY_5fbTzqRkYpyjcqQC4nBJ_3FuFkkun&amp;clipt=EJ_9TxiTwFM" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);
        $res->assertStatus(200);
        $this->assertDatabaseMissing('elements', ['source_url' => 
            '<iframe width="100%" height="270" src="https://www.youtube.com/embed/0SyTa7D62zQ?si=2Zt5VVfMc-bcESVN&amp;clip=UgkxTY_5fbTzqRkYpyjcqQC4nBJ_3FuFkkun&amp;clipt=EJ_9TxiTwFM&autoplay=1&playlist=0SyTa7D62zQ&loop=1" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>'
        ]);
    }

    public function testUpdateElementGfy()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        // update gfy url
        $url = 'https://gfycat.com/deficientunlinedgelding';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);
        $res->assertStatus(422);
        $this->assertDatabaseMissing('elements', ['source_url' => $url]);
    }

    public function testUpdateElementBilibili()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        
        $url = 'https://www.bilibili.com/video/BV1Zy4y1T7Zv';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);
        $res->assertStatus(200);
        $this->assertDatabaseHas('elements', ['source_url' => $url]);
    }

    public function testUpdateElementTwitch()
    {
        $post = $this->createPost();
        $element = $this->createElements($post, 1)[0];
        $this->be($post->user);

        
        $url = 'https://www.twitch.tv/videos/123456789';
        $res = $this->put(route('api.element.update', $element->id), [
            'post_serial' => $post->serial,
            'url' => $url,
        ], ['Accept' => 'application/json']);

        //todo
        $res->assertStatus(422);
        // $this->assertDatabaseHas('elements', ['source_url' => $url]);
    }

}