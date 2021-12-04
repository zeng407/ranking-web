<?php

namespace App\Http\Resources;

use App\Models\Game;
use App\Models\Game1V1Round;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameResource
 * @package App\Http\Resources
 * @mixin Game
 */
class GameResource extends JsonResource
{
    protected $elements;

    public function __construct(Game $game, $elements)
    {
        $this->elements = $elements;
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
        /** @var Game1V1Round $round */
        $round = $this->game_1v1_rounds()->latest('id')->first();

        if ($round === null) {
            $currentRound = 1;
            $ofRound = ceil($this->element_count / 2);
        } elseif ($round->current_round + 1 > $round->of_round) {
            $currentRound = 1;
            $ofRound = ceil($round->of_round / 2);
        } else {
            $currentRound = $round->current_round + 1;
            $ofRound = $round->of_round;
        }

        return [
            'title' => $this->post->title,
            'current_round' => $currentRound,
            'of_round' => $ofRound,
            'elements' => GameElementResource::collection($this->elements)
        ];
    }
}
