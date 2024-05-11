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


    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        $bilbiliService = new BilibiliService;
        $videoId = $bilbiliService->parseVideoId($sourceUrl);
        if (!$videoId) {
            return null;
        }

        $thumb = $bilbiliService->getThumbnail($sourceUrl);
        if($thumb){
            $path = $this->downloadImage($thumb, $serial)->getPath();
            logger("thumb path: $path");
            $thumb = Storage::url($path);
        }

        return [
            'title' => $params['title'] ?? $bilbiliService->getH1Title($sourceUrl),
            'thumb_url' => $thumb,
            'video_id' => $videoId,
            'path' => $path ?? null,
        ];
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
            'path' => $array['path'],
            'source_url' => $sourceUrl,
            'thumb_url' => $array['thumb_url'],
            'type' => ElementType::VIDEO,
            'title' => $array['title'],
            'video_source' => VideoSource::BILIBILI_VIDEO,
            'video_id' => $array['video_id'],
        ]);
    
        return $element;
    }
}