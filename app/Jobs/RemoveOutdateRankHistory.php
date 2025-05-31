<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Models\Post;
use App\Models\RankReport;
use App\Models\RankReportHistory;
use App\Services\RankService;
use Cache;
use Carbon\Carbon;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class RemoveOutdateRankHistory implements ShouldQueue, ShouldBeUnique
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
    public function handle()
    {
        RankReportHistory::where('post_id', $this->post->id)
            ->where('start_date', '<', now()->subDays(93))
            ->forceDelete();
    }

    public function uniqueId()
    {
        return $this->post->serial;
    }
}
