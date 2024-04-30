<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\BilibiliService;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;
use App\Services\TwitchService;


class TwitchElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        if(strpos($sourceUrl, '/videos/') !== false) {
            return $this->storeVideo($sourceUrl, $post, $params);
        }elseif(strpos($sourceUrl, '/clip/') !== false) {
            return $this->storeClip($sourceUrl, $post, $params);
        }
        return null;
    }

    protected function storeVideo(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $twitchService = new TwitchService;
        $videoId = $twitchService->parseVideoId($sourceUrl);
        logger("videoId ".$videoId);
        if (!$videoId) {
            return null;
        }
        try{
            $videoInfo = $twitchService->getVideo($videoId);
            $title = $videoInfo['user_name']. ' | '. $videoInfo['title'];
            $thumbnailUrl = $videoInfo['thumbnail_url'];
            $thumbnailUrl = str_replace(['%{width}', '%{height}'], [1024, 768], $thumbnailUrl);
        } catch (\Exception $e) {
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumbnailUrl ?? '',
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? $title ?? '',
            'video_source' => VideoSource::TWITCH_VIDEO,
            'video_id' => $videoId,
        ]);
    
        return $element;
    }

    protected function storeClip(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $twitchService = new TwitchService;
        $videoId = $twitchService->parseClipId($sourceUrl);
        logger("clip ".$videoId);
        if (!$videoId) {
            return null;
        }
        try{
            $videoInfo = $twitchService->getClip($videoId);
            $title = $videoInfo['broadcaster_name']. ' | '. $videoInfo['title'];
            $thumbnailUrl = $videoInfo['thumbnail_url'];
            $thumbnailUrl = str_replace(['%{width}', '%{height}'], [1024, 768], $thumbnailUrl);
        } catch (\Exception $e) {
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumbnailUrl ?? '',
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? $title ?? '',
            'video_source' => VideoSource::TWITCH_CLIP,
            'video_id' => $videoId,
        ]);
    
        return $element;
    }

}