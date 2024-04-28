<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;
use App\Services\YoutubeService;

class YoutubeElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        $video = app(YoutubeService::class)->query($sourceUrl);
        if (!$video) {
            return null;
        }
        try {
            $thumb = $video->getSnippet()->getThumbnails()->getHigh() ?:
                $video->getSnippet()->getThumbnails()->getMedium() ?:
                $video->getSnippet()->getThumbnails()->getStandard() ?:
                $video->getSnippet()->getThumbnails()->getMaxres() ?:
                $video->getSnippet()->getThumbnails()->getDefault();
            $thumbUrl = $thumb->getUrl();
            $title = $video->getSnippet()->getTitle();
            $id = $video->getId();
            $duration = $video->getContentDetails()->getDuration();
            preg_match('/^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$/', $duration, $parts);

            $hourPart = isset ($parts[1]) ? (int) $parts[1] : 0;
            $minutePart = isset ($parts[2]) ? (int) $parts[2] : 0;
            $secondPart = isset ($parts[3]) ? (int) $parts[3] : 0;
            $second = $hourPart * 3600 + $minutePart * 60 + $secondPart;

            $title = $params['title'] ?? $title;
            $element = $post->elements()->updateOrCreate(
                [
                    'source_url' => $params['old_source_url'] ?? '', // we don't replace old element if it's youtube video
                ],
                [
                    'source_url' => $sourceUrl,
                    'thumb_url' => $thumbUrl,
                    'title' => mb_substr($title, 0, config('setting.element_title_size')),
                    'type' => ElementType::VIDEO,
                    'video_source' => VideoSource::YOUTUBE,
                    'video_id' => $id,
                    'video_duration_second' => $second,
                ] + $params
            );

            return $element;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }
}