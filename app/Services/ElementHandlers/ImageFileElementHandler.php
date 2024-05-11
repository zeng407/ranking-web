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

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        //todo make thumb from video
        $thumb = $sourceUrl;
        $title = $params['title'] ?? 'untitled';
        $path = $params['path'] ?? null;

        return [
            'title' => $title,
            'thumb_url' => $thumb,
            'path' => $path,
        ];
    }

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $array = $this->storeArray($sourceUrl, $post->serial, $params);
        if(!$array){
            return null;
        }

        $element = $post->elements()->create([
            'path' => $array['path'],
            'source_url' => $sourceUrl,
            'thumb_url' => $array['thumb_url'],
            'type' => ElementType::IMAGE,
            'title' => $array['title'],
        ]);

        event(new ImageElementCreated($element, $post));
        return $element;
    }
}