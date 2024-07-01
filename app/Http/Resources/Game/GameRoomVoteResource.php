<?php

namespace App\Http\Resources\Game;

use App\Helper\CacheService;
use App\Models\GameRoom;
use App\Services\GameService;
use Illuminate\Http\Resources\Json\JsonResource;

/**
 * @mixin GameRoom
 */
class GameRoomVoteResource extends JsonResource
{

    /**
     * Transform the resource into an array.
     *
     * @param \Illuminate\Http\Request $request
     * @return array|\Illuminate\Contracts\Support\Arrayable|\JsonSerializable
     */
    public function toArray($request)
    {
        $round = $this->game->game_1v1_rounds()->orderByDesc('id')->first();
        if($round){
            $remainElements = $round->remain_elements;
        }else{
            $remainElements = $this->game->element_count;
        }
        $candidates = explode(',', $this->game->candidates);
        if(count($candidates) != 2){
            return [];
        }
        $firstCandidateVotes = $this->bets()->where('remain_elements', $remainElements)
            ->where('winner_id', $candidates[0])
            ->where('loser_id', $candidates[1])
            ->count();
        $secondCandidateVotes = $this->bets()->where('remain_elements', $remainElements)
            ->where('winner_id', $candidates[1])
            ->where('loser_id', $candidates[0])
            ->count();
        return [
            "first_candidate" => $candidates[0],
            "second_candidate" => $candidates[1],
            "first_candidate_votes" => $firstCandidateVotes,
            "second_candidate_votes" => $secondCandidateVotes,
            'remain_elements' => $remainElements,
            'total_votes' => $firstCandidateVotes + $secondCandidateVotes
        ];
    }
}
