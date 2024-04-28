<?php

use App\Models\User;
use App\Services\ElementHandlers\ImageElementHandler;
use App\Services\ElementService;
use App\Services\PostService;
use Tests\TestCase;
use App\Services\ImgurService;

class ImgurTest extends TestCase
{
    public function setUp(): void
    {
        parent::setUp();
        config(['services.imgur.enabled' => true]);
    }

    public function testGetAccountInfo()
    {
        $username = 'anyusername';
        $expectedResult = ['data' => ['id' => 123, 'username' => $username]];

        Http::fake([
            'account/'.$username => Http::response($expectedResult, 200),
        ]);

        $service = new ImgurService();

        $result = $service->getAccountInfo($username);
    
        $this->assertEquals($expectedResult, $result);
    }

    public function testCreateAlbum()
    {
        $title = 'anytitle';
        $description = 'anydescription';
        $expectedResult = ['data' => ['id' => 'QFH735d', 'deletehash' => 'pPEHoApUaxS220Q'], 'success' => true, 'status' => 200];

        Http::fake([
            'album' => Http::response($expectedResult, 200),
        ]);

        $service = new ImgurService();

        $result = $service->createAlubm($title, $description);
    
        $this->assertEquals($expectedResult, $result);
    }

    public function testCreateAlbumFail()
    {
        $expectedResult = ['data' => ['id' => 'QFH735d', 'deletehash' => 'pPEHoApUaxS220Q'], 'success' => false, 'status' => 200];

        Http::fake([
            'album' => Http::response($expectedResult, 200),
        ]);

        config(['app.env' => 'production']);

        /** @var User */
        $user = User::factory()->create();
        $postService = app(PostService::class);

        //todo run job for creating imgur album

        // $this->expectException(Exception::class);
        // $this->expectExceptionMessage('Failed to create album');

        $result = $postService->create($user, ['title' => 'anytitle', 'description' => 'anydescription']);
    }

    public function testCreateImage()
    {
        $imgUrl = 'anyurl';
        $title = 'anytitle';
        $description = 'anydescription';
        $albumId = 'anyalbumid';
        $expectedResult = ['data' => ['id' => 'QFH735d', 'deletehash' => 'pPEHoApUaxS220Q', 'link' => 'anylink'], 'success' => true, 'status' => 200];

        Http::fake([
            'image' => Http::response($expectedResult, 200),
        ]);

        $service = new ImgurService();

        $result = $service->uploadImage($imgUrl, $title, $description, $albumId);
    
        $this->assertEquals($expectedResult, $result);
    }

    public function testCreateImageFailNoImgurAblum()
    {
        $expectedResult = ['data' => ['id' => 'QFH735d', 'deletehash' => 'pPEHoApUaxS220Q', 'link' => 'anylink'], 'success' => false, 'status' => 200];

        Http::fake([
            'image' => Http::response($expectedResult, 200),
        ]);

        $user = User::factory()->create();
        $post = $user->posts()->create(['serial' => 'anyserial']);
        $service = app(ElementService::class);

        //todo run job for creating imgur album

        // $this->expectException(Exception::class);
        // $this->expectExceptionMessage('Post has no imgur album');

        $url = Faker\Provider\Image::imageUrl();
        $path = 'any/path';
        $result = (new ImageElementHandler)->storeElement($url, $post);
    }

    public function testCreateImageFailUpload()
    {
        $expectedResult = ['data' => ['id' => 'QFH735d', 'deletehash' => 'pPEHoApUaxS220Q', 'link' => 'anylink'], 'success' => false, 'status' => 200];

        Http::fake([
            'image' => Http::response($expectedResult, 200),
        ]);

        $user = User::factory()->create();
        $post = $user->posts()->create(['serial' => 'anyserial']);
        $post->imgur_album()->create(['album_id' => 'anyalbumid', 'title' => 'anytitle', 'description' => 'anydescription']);
        $service = app(ElementService::class);
        
        //todo run job for creating imgur album

        // $this->expectException(Exception::class);
        // $this->expectExceptionMessage('Failed to upload image');

        $url = Faker\Provider\Image::imageUrl();
        
        $element = (new ImageElementHandler)->storeElement($url, $post);
        
        $this->assertNotNull($element);
        $this->assertTrue(\Storage::exists($element->path));
    }
}