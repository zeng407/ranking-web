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

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        try {
            $directory = $serial;
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

        return [
            'title' => $title,
            'thumb_url' => $localUrl,
            'path' => $storageImage->getPath(),
        ];
    }

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $array = $this->storeArray($sourceUrl, $post->serial, $params);
        if(!$array){
            return null;
        }

        $originalElement = $post->elements()
            ->where('source_url', $params['old_source_url'] ?? $sourceUrl)
            ->first();
        if($originalElement && $originalElement->thumb_url !== $array['thumb_url'] && $originalElement->imgur_image){
            $originalElement->imgur_image->delete();
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
            'path' => $array['path'],
            'source_url' => $sourceUrl,
            'thumb_url' => $array['thumb_url'],
            'mediumthumb_url' => null,
            'lowthumb_url' => null,
            'type' => ElementType::IMAGE,
            'title' => $array['title'],
        ]);

        event(new ImageElementCreated($element, $post));

        return $element;
    }
}
