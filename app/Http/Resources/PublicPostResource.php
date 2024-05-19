<?php

namespace App\Http\Resources;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Services\RankService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class PublicPostResource
 * @package App\Http\Resources
 * @mixin Post
 */
class PublicPostResource extends JsonResource
{
    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        /** @var RankService */
        $rankService = app(RankService::class);
        $ranks = collect($rankService->getRankReports($this->resource, 5, 1)->items());
        if($ranks->count() >= 2) {
            $ranks = $ranks->shuffle();
            $element1 = $ranks->pop()->element;
            $element2 = $ranks->pop()->element;
        }else{
            $elements = $this->elements->shuffle()->take(2);
            $element1 = $elements->pop();
            $element2 = $elements->pop();
        }
        $elementsCount = $this->elements->count();

        return [
            'title' => $this->title,
            'serial' => $this->serial,
            'is_private' => $this->isPrivate(),
            'description' => $this->description,
            'element1' => [
                'video_source' => $element1->video_source,
                'type' => $element1->type,
                'id' => $element1?->id,
                'url' => $element1?->thumb_url,
                'url2' => $element1?->imgur_image?->link,
                'title' => $element1?->title,
                'previewable'=> $this->isPreviewable($element1)
            ],
            'element2' => [
                'video_source' => $element2->video_source,
                'type' => $element2->type,
                'id' => $element2?->id,
                'url' => $element2?->thumb_url,
                'url2' => $element2?->imgur_image?->link,
                'title' => $element2?->title,
                'previewable'=> $this->isPreviewable($element2)
            ],
            'created_at' => $this->created_at,
            'updated_at' => $this->updated_at,
            'play_count' => $this->games()->count(),
            'elements_count' => $elementsCount,
            'tags' => $this->tags->pluck('name')->toArray(),
        ];
    }

    protected function isPreviewable(Element $element)
    {
        return $element->type === \App\Enums\ElementType::IMAGE || 
            $element->video_source === \App\Enums\VideoSource::YOUTUBE || 
            $element->video_source === \App\Enums\VideoSource::YOUTUBE_EMBED ||
            $element->video_source === \App\Enums\VideoSource::BILIBILI_VIDEO ||
            $element->video_source === \App\Enums\VideoSource::TWITCH_VIDEO ||
            $element->video_source === \App\Enums\VideoSource::TWITCH_CLIP ||
            ($element->video_source === \App\Enums\VideoSource::URL && $this->isImageType($element->thumb_url));
    }

    protected function isImageType($url)
    {
        // string is end with images type
        return preg_match('/\.(jpeg|jpg|png|gif|webp)$/', $url);
    }
}
