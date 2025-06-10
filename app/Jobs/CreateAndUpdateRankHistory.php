<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Helper\CacheService;
use App\Models\Post;
use App\Models\RankReport;
use App\Services\RankService;
use Cache;
use Carbon\Carbon;
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

    protected $refresh;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Post $post, $refresh = false)
    {
        $this->post = $post;
        $this->refresh = $refresh;
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
        logger('CreateAndUpdateRankHistory job fired for post id: ' . $this->post->id);
        $counter = 0;
        RankReport::setEagerLoads([])->where('post_id', $this->post->id)->eachById(function ($report) use(&$counter){
            $counter++;
            CacheService::putRankHistoryJobCache($this->post->id, $counter);
            CreateRankReportHistory::dispatch($report, $this->refresh, today()->subDays(1)->toDateString());
        });

        ReorderRankReportHistory::dispatch($this->post)->delay(now()->addSeconds(5));
        CacheService::clearNeedFreshPostRank($this->post);
    }

    public function uniqueId()
    {
        return $this->post->serial;
    }
}
