<?php

namespace App\Listeners;

use App\Events\ElementDeleted;
use App\Events\PostDeleted;
use App\Jobs\UpdateRankReport;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Services\RankService;

class DeletePostRank
{
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
     * @param  PostDeleted  $event
     * @return void
     */
    public function handle(PostDeleted $event)
    {
        logger('DeletePostRank listener fired');
        $post = $event->getPost();

        RankReport::where('post_id', $post->id)->delete();
        UpdateRankReport::dispatch($post)->delay(10);
    }
}
