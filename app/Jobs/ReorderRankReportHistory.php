<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Helper\CacheService;
use App\Models\Post;
use App\Services\RankService;
use Cache;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class ReorderRankReportHistory implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected Post $post;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Post $post)
    {
        $this->post = $post;
        $this->onQueue('rank_report_history');
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle(RankService $rankService)
    {
        logger('ReorderRankReportHistory job fired for post id: ' . $this->post->id);
        $count = CacheService::getRankHistoryJobCache($this->post->id);
        if ($count > 0) {
            logger('ReorderRankReportHistory job skipped for post id: ' . $this->post->id, ['count' => $count]);
            // If the count is not zero, it means previous jobs are still running
            // so we will not proceed with this job

            // put the job back to the queue with a delay
            $this->dispatch($this->post)->delay(now()->addSeconds(60));
            return;
        }

        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::ALL);
        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::WEEK);

        logger('ReorderRankReportHistory job completed for post id: ' . $this->post->id);
    }

}
