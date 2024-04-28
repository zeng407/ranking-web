<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\BilibiliService;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;
use Storage;

class BilibiliElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $bilbiliService = new BilibiliService;
        $videoId = $bilbiliService->parseVideoId($sourceUrl);
        if (!$videoId) {
            return null;
        }

        $thumb = $bilbiliService->getThumbnail($sourceUrl);
        if($thumb){
            $path = $this->downloadImage($thumb, $post->serial)->getPath();
            logger("thumb path: $path");
            $thumb = Storage::url($path);
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'path' => $path ?? null,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumb,
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? $bilbiliService->getH1Title($sourceUrl),
            'video_source' => VideoSource::BILIBILI_VIDEO,
            'video_id' => $videoId,
        ]);
    
        return $element;
    }
}