<?php

namespace App\Listeners;

use App\Events\ElementDeleted;
use App\Jobs\UpdateRankReport;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Services\RankService;

class DeleteElementRank
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
     * @param  ElementDeleted  $event
     * @return void
     */
    public function handle(ElementDeleted $event)
    {
        logger('DeleteElementRank listener fired');
        $element = $event->getElement();

        $posts = RankReport::where('element_id', $element->id)->pluck('post_id');
        RankReport::where('element_id', $element->id)->delete();

        foreach ($posts as $postId) {
            $post = Post::find($postId);
            UpdateRankReport::dispatch($post)
                ->delay(10);
        }
    }
}
