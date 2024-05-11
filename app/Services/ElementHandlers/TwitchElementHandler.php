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

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        if(strpos($sourceUrl, '/videos/') !== false) {
            return $this->storeVideoArray($sourceUrl, $serial, $params);
        }elseif(strpos($sourceUrl, '/clip/') !== false) {
            return $this->storeClipArray($sourceUrl, $serial, $params);
        }
        return null;
    }
    
    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $array = $this->storeArray($sourceUrl, $post->serial, $params);
        if(!$array){
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => $array['thumb_url'],
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? $array['title'],
            'video_source' => $array['video_source'],
            'video_id' => $array['video_id'],
        ]);
    
        return $element;
    }

    protected function storeVideoArray(string $sourceUrl, string $serial, $params = []) : ?array
    {
        $twitchService = new TwitchService;
        $videoId = $twitchService->parseVideoId($sourceUrl);
        logger("videoId ".$videoId);
        if (!$videoId) {
            return null;
        }

        try{
            $start = $params['video_start_second'] ?? null;
            $videoInfo = $twitchService->getVideo($videoId);
            $title = $videoInfo['user_name']. ' | '. $videoInfo['title'];
            $thumbnailUrl = $videoInfo['thumbnail_url'];
            $thumbnailUrl = str_replace(['%{width}', '%{height}'], [1024, 768], $thumbnailUrl);
        } catch (\Exception $e) {
            logger($e->getMessage());
            return null;
        }

        return [
            'title' => $title,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumbnailUrl,
            'video_id' => $videoId,
            'video_source' => VideoSource::TWITCH_VIDEO,
            'video_start_second' => $start
        ];
    }


    protected function storeClipArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        $twitchService = new TwitchService;
        $videoId = $twitchService->parseClipId($sourceUrl);
        logger("clip ".$videoId);
        if (!$videoId) {
            return null;
        }
        try{
            $start = $params['video_start_second'] ?? null;
            $videoInfo = $twitchService->getClip($videoId);
            $title = $videoInfo['broadcaster_name']. ' | '. $videoInfo['title'];
            $thumbnailUrl = $videoInfo['thumbnail_url'];
            $thumbnailUrl = str_replace(['%{width}', '%{height}'], [1024, 768], $thumbnailUrl);
        } catch (\Exception $e) {
            logger($e->getMessage());
            return null;
        }

        return [
            'title' => $title,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumbnailUrl,
            'video_id' => $videoId,
            'video_source' => VideoSource::TWITCH_CLIP,
            'video_start_second' => $start
        ];
    }

}