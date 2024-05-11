<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;

class VideoUrlElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        //todo make thumb from video
        $title = $params['title'] ?? 'video';

        return [
            'title' => $title,
            'thumb_url' => $sourceUrl,
            'source_url' => $sourceUrl,
        ];
    }
    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $array = $this->storeArray($sourceUrl, $post->serial, $params);
        if(!$array){
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ?? $sourceUrl,
        ], [
            'path' => null,
            'source_url' => $array['source_url'],
            'thumb_url' => $array['thumb_url'],
            'type' => ElementType::VIDEO,
            'title' => $array['title'],
            'video_source' => VideoSource::URL
        ]);

        return $element;
    }
}