<?php

namespace App\Http\Resources;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Services\RankService;
use Illuminate\Http\Resources\Json\JsonResource;

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
        $data = $this->data;

        return [
            'title' => $data['title'],
            'serial' => $data['serial'],
            'is_private' => $data['is_private'],
            'description' => $this->description,
            'element1' => $data['element1'],
            'element2' => $data['element2'],
            'created_at' => $data['created_at'],
            'updated_at' => $data['updated_at'],
            'play_count' => $data['play_count'],
            'elements_count' => $data['elements_count'],
            'tags' => json_decode($this->tags, true),
            'is_censored' => $data['is_censored'],
        ];
    }

}
