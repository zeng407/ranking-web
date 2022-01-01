<?php

namespace App\Http\Resources;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
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
     * @param  \Illuminate\Http\Request  $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $image1 = $this->getImage1();
        $image2 = $this->getImage2();

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
            'updated_at' => $this->updated_at
        ];
    }

    protected function getImage1():?Element
    {
        $top = $this->rank_reports()
            ->orderByRaw('ifnull(final_win_position,9999)')
            ->first();
        if ($top) {
            return $top->element;
        }

        return $this->elements()->inRandomOrder()->first();
    }

    protected function getImage2():?Element
    {
        $top = $this->rank_reports()
            ->orderByRaw('ifnull(final_win_position,9999)')
            ->skip(1)->first();
        if ($top) {
            return $top->element;
        }

        return $this->elements()->inRandomOrder()->first();
    }
}
