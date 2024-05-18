<?php

namespace App\Listeners;

use App\Events\GameComplete;
use App\Jobs\UpdateRankReport;
use App\Services\RankService;
use Illuminate\Bus\Queueable;

class UpdatePostRank
{
    use Queueable;

    protected RankService $rankService;

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
        logger('[UpdatePostRank] listener handle', ['post_id' => $event->post->id]);

        //delay 10 seconds for the GameVoted event to be fully processed
        UpdateRankReport::dispatch($event->post)->delay(10);
    }
}
