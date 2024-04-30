<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Events\ImageElementCreated;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;
use Storage;

class ImageFileElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        //todo make thumb from video
        $thumb = $sourceUrl;
        $title = $params['title'] ?? 'untitled';
        $path = $params['path'] ?? null;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => $title,
        ]);

        event(new ImageElementCreated($element, $post));
        return $element;
    }
}