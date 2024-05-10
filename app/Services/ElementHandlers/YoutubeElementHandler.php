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

    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
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
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return [
            'title' => $title,
            'thumb_url' => $thumbUrl,
            'source_url' => $sourceUrl,
            'video_id' => $id,
            'video_duration_second' => $second,
            'video_start_second' => $params['video_start_second'] ?? null,
        ];
    }

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $array = $this->storeArray($sourceUrl, $post->serial, $params);
            if (!$array) {
                return null;
            }
            $element = $post->elements()->updateOrCreate(
                [
                    'source_url' => $params['old_source_url'] ?? '', // we don't replace old element if it's youtube video
                ],
                [
                    'source_url' => $sourceUrl,
                    'thumb_url' => $array['thumb_url'],
                    'title' =>  mb_substr($array['title'], 0, config('setting.element_title_size')),
                    'type' => ElementType::VIDEO,
                    'video_source' => VideoSource::YOUTUBE,
                    'video_id' => $array['video_id'],
                    'video_duration_second' => $array['video_duration_second'],
                ] + $params
            );

            return $element;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }
}