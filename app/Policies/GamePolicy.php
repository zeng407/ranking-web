<?php

namespace App\Policies;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\User;
use App\Services\GameService;
use Illuminate\Auth\Access\HandlesAuthorization;

class GamePolicy
{
    use HandlesAuthorization;

    protected $gameService;

    public function __construct(GameService $gameService)
    {
        $this->gameService = $gameService;
    }


    public function play(?User $user, Game $game)
    {
        if($user && $user->id === $game->post->user_id){
            return true;
        }

        return $this->gameService->isGamePublic($game)
            && !$this->gameService->isGameComplete($game);
    }
}

