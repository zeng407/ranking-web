<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Jobs\BroadcastNewChampion;
use App\Jobs\CacheChampion;
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

        if($event->post->isPublic() && $event->post->is_censored == false) {
            CacheChampion::dispatch();
            broadcast(new BroadcastNewChampion($userGameResult));
        }
    }
}
