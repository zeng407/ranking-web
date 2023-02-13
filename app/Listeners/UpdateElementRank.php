<?php

namespace App\Listeners;

use App\Events\GameElementVoted;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;

class UpdateElementRank implements ShouldQueue
{
    use Queueable;

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

    public function shouldQueue(GameElementVoted $event)
    {
        return !$event->isFinal;
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
