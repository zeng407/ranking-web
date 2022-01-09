<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Services\RankService;

class UpdatePostRank
{
    protected $rankService;

    /**
     * Create the event listener.
     *
     * @return void
     */
    public function __construct(RankService $rankService)
    {
        $this->rankService = $rankService;
    }

    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(GameComplete $event)
    {
        $this->rankService->createRankReport($event->game->post);
    }
}
