<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Events\VideoElementCreated;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;

class VideoUrlElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
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

        // find old_element and delete files
        if ($params['old_source_url'] ?? null) {
            $oldElement = $post->elements()->where('source_url', $params['old_source_url'])->first();
            if ($oldElement) {
                $this->deleteElemntFile($oldElement->path);
                $this->deleteElemntFile($oldElement->thumb_url);
                $this->deleteElemntFile($oldElement->lowthumb_url);
                $this->deleteElemntFile($oldElement->mediumthumb_url);
            }
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

        event(new VideoElementCreated($element, $post));

        return $element;
    }
}
