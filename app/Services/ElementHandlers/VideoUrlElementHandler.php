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

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        //todo make thumb from video
        $title = $params['title'] ?? 'video';

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ?? $sourceUrl,
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => $sourceUrl,
            'type' => ElementType::VIDEO,
            'title' => $title,
            'video_source' => VideoSource::URL
        ]);

        return $element;
    }
}