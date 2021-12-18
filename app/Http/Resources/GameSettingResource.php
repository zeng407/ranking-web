<?php

namespace App\Http\Resources;

use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameSettingResource
 * @package App\Http\Resources
 * @mixin Post
 */
class GameSettingResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $elementsCount = $this->elements()->count();
        return [
            'title' => $this->title,
            'elements_count' => $elementsCount
        ];
    }
}
