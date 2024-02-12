<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;

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
        logger('[UpdatePostRank] listener handle', ['game_id' => $event->game->id]);
        $this->rankService->createRankReport($event->game->post);
    }
}
