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
use Illuminate\Queue\Middleware\WithoutOverlapping;


class ReorderRankReportHistory implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected Post $post;
    protected $nextdelay;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Post $post, $delay = 60)
    {
        $this->post = $post;
        $this->nextdelay = $delay;
        $this->onQueue('rank_report_history');
    }

    public function middleware(): array
    {
        return [(new WithoutOverlapping($this->post->serial))->dontRelease()];
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

            // put the job back to the queue with a delay
            $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::ALL);
            $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::THOUSAND_VOTES);
            $this->dispatch($this->post, $this->nextdelay + 60)->delay(now()->addSeconds($this->nextdelay));
            return;
        }

        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::ALL);
        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::THOUSAND_VOTES);

        logger('ReorderRankReportHistory job completed for post id: ' . $this->post->id);
    }

}
