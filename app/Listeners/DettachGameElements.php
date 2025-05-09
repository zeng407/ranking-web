<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Events\GameElementVoted;
use App\Services\RankService;
use App\Jobs\DettachGameElements as DettachGameElementsJob;

class DettachGameElements
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
        DettachGameElementsJob::dispatch($event->game);
    }
}
