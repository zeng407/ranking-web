<?php

namespace App\Http\Resources\MyPost;

use App\Http\Resources\Game\GameElementResource;
use App\Models\RankReport;
use Illuminate\Http\Resources\Json\JsonResource;


/**
 * Class PostRankResource
 * @package App\Http\Resources\Post
 * @mixin RankReport
 */
class PostRankResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        return [
            'element' => GameElementResource::make($this->element),
            'final_win_position' => $this->final_win_position,
            'final_win_rate' => round($this->final_win_rate,1),
            'win_position' => $this->win_position,
            'win_rate' => round($this->win_rate,1),
            'rank' => $this->rank
        ];
    }
}
