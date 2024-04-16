<?php

namespace App\Listeners;

use App\Events\GameElementVoted;
use App\Services\RankService;

class UpdateElementRank 
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
    public function handle(GameElementVoted $event)
    {
        \App\Jobs\UpdateElementRank::dispatch($event->game, $event->element);
    }
}
