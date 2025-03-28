<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
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
            Cache::put('CreateAndUpdateRankHistory:' . $this->post->id, $counter, Carbon::now()->addMinutes(10));
            CreateRankReportHistory::dispatch($report, $this->refresh, $this->post->created_at);
        });

        ReorderRankReportHistory::dispatch($this->post)->delay(now()->addSeconds(5));
    }

    public function uniqueId()
    {
        return $this->post->serial;
    }
}
