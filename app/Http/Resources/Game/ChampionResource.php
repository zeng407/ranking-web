<?php

namespace App\Http\Resources\Game;

use App\Models\UserGameResult;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameRoundResource
 * @package App\Http\Resources
 * @mixin UserGameResult
 */
class ChampionResource extends JsonResource
{

    public function __construct(UserGameResult $game)
    {
        parent::__construct($game);
    }

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $candidates = explode(',', $this->candidates);
        $left = $this->getElement(isset($candidates[0]) && $candidates[0] ? $candidates[0] : $this->champion_id);
        $right = $this->getElement(isset($candidates[1]) && $candidates[1] ? $candidates[1] : $this->loser_id);
        return [
            'post_title' => $this->game->post->title,
            'game_serial' => $this->game->serial,
            'left' => $left,
            'right' => $right,
            'datetime' => $this->created_at,
            'thumb_url' => $this->champion->thumb_url,
            'key' => md5($this->id),
        ];
    }

    protected function getElement($id)
    {
        if($id == $this->champion_id) {
            return [
                'name' => $this->champion_name,
                'thumb_url' => $this->champion->thumb_url,
                'is_winner' => true,
            ];
        }elseif($id == $this->loser_id) {
            return [
                'name' => $this->loser_name,
                'thumb_url' => $this->loser?->thumb_url,
                'is_winner' => false,
            ];
        }
    }
}