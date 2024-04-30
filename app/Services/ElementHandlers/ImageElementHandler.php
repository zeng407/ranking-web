<?php

namespace App\Services\ElementHandlers;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use Storage;
use App\Enums\ElementType;
use App\Events\ImageElementCreated;

class ImageElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $directory = $post->serial;
            $storageImage = $this->downloadImage($sourceUrl, $directory);

            if ($storageImage === null) {
                return null;
            }
            $fileInfo = $storageImage->getFileInfo();
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        $title = $params['title'] ?? $fileInfo['filename'];
        $localUrl = Storage::url($storageImage->getPath());

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ?? $sourceUrl,
        ], [
            'path' => $storageImage->getPath(),
            'source_url' => $sourceUrl,
            'thumb_url' => $localUrl,
            'type' => ElementType::IMAGE,
            'title' => $title
        ]);
        event(new ImageElementCreated($element, $post));

        return $element;
    }
}