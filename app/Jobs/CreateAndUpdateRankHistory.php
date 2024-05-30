<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Models\Post;
use App\Models\RankReport;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class CreateAndUpdateRankHistory implements ShouldQueue, ShouldBeUnique
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
        $this->delay(now()->addSeconds(10));
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle(RankService $rankService)
    {
        logger('CreateAndUpdateRankHistory job fired for post id: ' . $this->post->id);
        RankReport::with('post')->where('post_id', $this->post->id)->eachById(function ($report) use($rankService){
            $rankService->createRankReportHistory($report, RankReportTimeRange::ALL);
            $rankService->createRankReportHistory($report, RankReportTimeRange::WEEK);
        });

        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::ALL);
        $rankService->updateRankReportHistoryRank($this->post, RankReportTimeRange::WEEK);
    }

    public function uniqueId()
    {
        return $this->post->serial;
    }
}
