<?php

namespace Tests\Unit;

use Tests\TestCase;
use Tests\TestHelper;
use App\Services\ElementSourceGuess;

class ElementSourceGuessTest extends TestCase
{
    use TestHelper;

    public function testGuessImageUrl()
    {
        $guess = new ElementSourceGuess();
        $sources = [
                    'https://example.com/image.jpg'
        ];
        foreach ($sources as $source) {
            $guess->guess($source);
            $this->assertTrue($guess->isImage);
        }

        $sources = [
            'https://upload.wikimedia.org/wikipedia/en/a/a9/Example.jpg?20240301091138',
            'https://upload.wikimedia.org/wikipedia/commons/7/70/Example.png',
            'https://www.easygifanimator.net/images/samples/eglite.gif',
            'https://i.imgur.com/8nLFCVP.png'
        ];
        foreach ($sources as $source) {
            $guess->guess($source);
            $this->assertTrue($guess->isImage);
            $this->assertFalse($guess->isImgur);
            $this->assertFalse($guess->isVideo);
            $this->assertFalse($guess->isYoutube);
            $this->assertNull($guess->youtubeId);
            $this->assertFalse($guess->isGFY);
            $this->assertFalse($guess->isBilibili);
            $this->assertFalse($guess->isTwitch);
            $this->assertFalse($guess->isYoutubeEmbed);
        }
    }

    public function testGuessImgurUrl()
    {
        $sources = [
            'https://imgur.com/gallery/8nLFCVP',
            'https://imgur.com/gallery/yGhDZJ5',
            'https://imgur.com/t/birds/W7Mod3p'
        ];

        $guess = new ElementSourceGuess();
        foreach ($sources as $source) {
            $guess->guess($source);
            $this->assertFalse($guess->isImage);
            $this->assertTrue($guess->isImgur);
            $this->assertFalse($guess->isVideo);
            $this->assertFalse($guess->isYoutube);
            $this->assertNull($guess->youtubeId);
            $this->assertFalse($guess->isGFY);
            $this->assertFalse($guess->isBilibili);
            $this->assertFalse($guess->isTwitch);
            $this->assertFalse($guess->isYoutubeEmbed);
        }
    }

    public function testGuessVideoUrl()
    {
        $source = 'http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4';
        $guess = new ElementSourceGuess();
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertTrue($guess->isVideo);
        $this->assertFalse($guess->isYoutube);
        $this->assertNull($guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);
    }

    public function testGuessYoutube()
    {
        $source = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ';
        $guess = new ElementSourceGuess();
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertTrue($guess->isYoutube);
        $this->assertEquals('dQw4w9WgXcQ', $guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);

        $source = '<iframe width="100%" height="270" src="https://www.youtube.com/embed/0SyTa7D62zQ?si=2Zt5VVfMc-bcESVN&amp;clip=UgkxTY_5fbTzqRkYpyjcqQC4nBJ_3FuFkkun&amp;clipt=EJ_9TxiTwFM" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen></iframe>';
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertFalse($guess->isYoutube);
        $this->assertEquals('0SyTa7D62zQ', $guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertTrue($guess->isYoutubeEmbed);

        $source = 'https://www.youtube.com/shorts/-YGfTA0qFf0';
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertTrue($guess->isYoutube);
        $this->assertEquals('-YGfTA0qFf0', $guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);

        $source = 'https://youtube.com/clip/Ugkx6JO_hA-tbdCBm5saXncf775k5h5ZggF5?si=psJWy2AnYwVq9RW0';
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertTrue($guess->isYoutube);
        $this->assertEquals('Ugkx6JO_hA-tbdCBm5saXncf775k5h5ZggF5', $guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);
    }

    public function testGuessGfy()
    {
        $source = 'https://gfycat.com/someslug';
        $guess = new ElementSourceGuess();
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertFalse($guess->isYoutube);
        $this->assertNull($guess->youtubeId);
        // $this->assertTrue($guess->isGFY);
        $this->assertFalse($guess->isGFY); // gfycat service is not available now
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);
    }

    public function testGuessBilibili()
    {
        $source = 'https://www.bilibili.com/video/BV1Kb411W75N';
        $guess = new ElementSourceGuess();
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertFalse($guess->isYoutube);
        $this->assertNull($guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertTrue($guess->isBilibili);
        $this->assertFalse($guess->isTwitch);
        $this->assertFalse($guess->isYoutubeEmbed);
    }

    public function testGuessTwitch()
    {
        $source = 'https://www.twitch.tv/somechannel';
        $guess = new ElementSourceGuess();
        $guess->guess($source);
        $this->assertFalse($guess->isImage);
        $this->assertFalse($guess->isImgur);
        $this->assertFalse($guess->isVideo);
        $this->assertFalse($guess->isYoutube);
        $this->assertNull($guess->youtubeId);
        $this->assertFalse($guess->isGFY);
        $this->assertFalse($guess->isBilibili);
        $this->assertFalse($guess->isYoutubeEmbed);
        $this->assertTrue($guess->isTwitch);
    }



}
