<?php

namespace App\Http\Resources;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Services\RankService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class PostResource
 * @package App\Http\Resources
 * @mixin Post
 */
class PostResource extends JsonResource
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

        return [
            'title' => $this->title,
            'serial' => $this->serial,
            'is_private' => $this->isPrivate(),
            'description' => $this->description,
            'element1' => $this->getElement($element1),
            'element2' => $this->getElement($element2),
            'created_at' => $this->created_at->toDateTimeString(),
            'updated_at' => $this->updated_at->toDateTimeString(),
            'play_count' => $this->games()->count(),
            'elements_count' => $this->elements()->count(),
            'tags' => $this->tags->pluck('name')->toArray(),
            'is_censored' => $this->is_censored,
        ];
    }


    protected function getElement(?Element $element)
    {
        if(!$element) return [
            'video_source' => null,
            'type' => null,
            'id' => null,
            'url' => null,
            'url2' => null,
            'title' => null,
            'previewable'=> false
        ];

        return [
            'video_source' => $element->video_source,
            'type' => $element->type,
            'id' => $element->id,
            'url' => $element->getLowThumbUrl(),
            'url2' => $element->imgur_image?->link,
            'title' => $element->title,
            'previewable'=> $this->isPreviewable($element)
        ];
    }

    protected function isPreviewable(?Element $element)
    {
        if (!$element) {
            return false;
        }
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
