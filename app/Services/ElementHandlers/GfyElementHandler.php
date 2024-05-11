<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\GfycatService;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;

class GfyElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        $gfycatService = app(GfycatService::class);
        $id = $gfycatService->getId($sourceUrl);
        $info = $gfycatService->getInfo($id);

        $title = $params['title'] ?? $info->gfyItem->title;

        return [
            'title' => $title,
            'thumb_url' => $info->gfyItem->posterUrl,
            'source_url' => $info->gfyItem->mp4Url,
        ];
    }

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $array = $this->storeArray($sourceUrl, $post->serial, $params);

            $element = $post->elements()->updateOrCreate([
                'source_url' => $params['old_source_url'] ?? $array['source_url'],
            ], [
                'source_url' => $array['source_url'],
                'thumb_url' => $array['thumb_url'],
                'type' => ElementType::VIDEO,
                'title' => $array['title'],
                'video_source' => VideoSource::GFYCAT
            ]);

        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return $element;
    }
}