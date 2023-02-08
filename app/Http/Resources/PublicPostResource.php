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
        $ranks = collect(app(RankService::class)->getRankReports($this->resource, 5)->items());
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
            'description' => $this->description,
            'image1' => [
                'url' => optional($image1)->thumb_url,
                'title' => optional($image1)->title
            ],
            'image2' => [
                'url' => optional($image2)->thumb_url,
                'title' => optional($image2)->title
            ],
            'created_at' => $this->created_at,
            'updated_at' => $this->updated_at,
            'play_count' => $this->games()->count(),
            'elements_count' => $elementsCount
        ];
    }
}
