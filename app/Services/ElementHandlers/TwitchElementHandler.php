<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\BilibiliService;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;
use App\Services\TwitchService;
use Storage;

class TwitchElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $twitchService = new TwitchService;
        $videoId = $twitchService->parseVideoId($sourceUrl);
        if (!$videoId) {
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => null,
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? '',
            'video_source' => VideoSource::TWITCH,
            'video_id' => $videoId,
        ]);
    
        return $element;
    }
}