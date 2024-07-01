<?php

namespace App\Http\Resources\Game;

use App\Models\Game;
use App\Models\Game1V1Round;
use App\Services\GameService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * Class GameRoundResource
 * @package App\Http\Resources
 * @mixin Game
 */
class GameRoundResource extends JsonResource
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
        $remains = $round ? $round->remain_elements : $this->element_count;

        if ($round === null) {
            $currentRound = 1;
            $ofRound = ceil($this->element_count / 2);
        } elseif ($round->current_round + 1 > $round->of_round) {
            $currentRound = 1;
            $ofRound = app(GameService::class)->calculateNextRoundNumber($remains);
        } else {
            $currentRound = $round->current_round + 1;
            $ofRound = $round->of_round;
        }

        return [
            'hash' => game_round_hash($currentRound, $ofRound, $remains, collect($this->elements)->pluck('id')->toArray()),
            'title' => $this->post->title,
            'current_round' => $currentRound,
            'of_round' => $ofRound,
            'elements' => GameElementResource::collection($this->elements),
            'remain_elements' => $remains,
            'total_elements' => $this->element_count
        ];
    }
}
