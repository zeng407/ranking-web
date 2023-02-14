<?php

namespace App\Listeners;

use App\Events\GameElementVoted;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;

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
        $this->rankService->createElementRank($event->game, $event->element);
    }
}
