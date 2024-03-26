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
        $ranks = collect($rankService->getRankReports($this->resource, 5)->items());
        if($ranks->count() >= 2) {
            $ranks = $ranks->shuffle();
            $image1 = $ranks->pop()->element;
            $image2 = $ranks->pop()->element;
        }else{
            $elements = $this->elements()->inRandomOrder()->take(2)->get();
            $image1 = $elements->pop();
            $image2 = $elements->pop();
        }
        $elementsCount = $this->elements()->count();

        return [
            'title' => $this->title,
            'serial' => $this->serial,
            'is_private' => $this->isPrivate(),
            'description' => $this->description,
            'image1' => [
                'id' => $image1?->id,
                'url' => $image1?->thumb_url,
                'url2' => $image1?->imgur_image?->link,
                'title' => $image1?->title,

            ],
            'image2' => [
                'id' => $image2?->id,
                'url' => $image2?->thumb_url,
                'url2' => $image2?->imgur_image?->link,
                'title' => $image2?->title
            ],
            'created_at' => $this->created_at,
            'updated_at' => $this->updated_at,
            'play_count' => $this->games()->count(),
            'elements_count' => $elementsCount,
            'tags' => $this->tags->pluck('name')->toArray(),
        ];
    }
}
