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

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $gfycatService = app(GfycatService::class);
            $id = $gfycatService->getId($sourceUrl);
            $info = $gfycatService->getInfo($id);

            $title = $params['title'] ?? $info->gfyItem->title;

            $element = $post->elements()->updateOrCreate([
                'source_url' => $params['old_source_url'] ?? $info->gfyItem->mp4Url,
            ], [
                'source_url' => $info->gfyItem->mp4Url,
                'thumb_url' => $info->gfyItem->posterUrl,
                'type' => ElementType::VIDEO,
                'title' => $title,
                'video_source' => VideoSource::GFYCAT
            ]);

        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return $element;
    }
}