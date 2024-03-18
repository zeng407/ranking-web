<?php

namespace App\Listeners;

use App\Events\GameComplete;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
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
        $this->gameService->createVotedChampion($event->user, $event->anonymousId, $event->game, $event->champion);
    }
}
