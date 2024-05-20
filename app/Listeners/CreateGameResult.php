<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Jobs\BroadcastNewChampion;
use App\Services\GameService;

class CreateGameResult
{
    private GameService $gameService;
    /**
     * Create the event listener.
     *
     * @return void
     */
    public function __construct(GameService $gameService)
    {
        $this->gameService = $gameService;
    }
    
    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(GameComplete $event)
    {
        $userGameResult = $this->gameService->createUserGameResult(
            $event->user,
            $event->anonymousId, 
            $event->gameRound,
            $event->candidates
        );
        
        if($event->post->isPublic()) {
            broadcast(new BroadcastNewChampion($userGameResult));
        }
    }
}
