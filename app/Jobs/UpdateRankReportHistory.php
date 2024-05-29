<?php

namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Models\RankReport;
use App\Models\RankReportHistory;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class UpdateRankReportHistory implements ShouldQueue
{
    use Dispatchable, SerializesModels, Queueable, InteractsWithQueue;

    protected int $postId;
    protected RankReportTimeRange $timeRange;
    protected string $startDate;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(int $postId, RankReportTimeRange $timeRange, string $startDate)
    {
        $this->postId = $postId;
        $this->timeRange = $timeRange;
        $this->startDate = $startDate;
        $this->onQueue('rank_report_history');
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        logger('UpdateRankReportHistory job fired postId: ' .
            $this->postId . ' timeRange: ' . $this->timeRange->value . ' startDate: ' . $this->startDate);
        $counter = 0;
        RankReportHistory::where('post_id', $this->postId)
            ->where('time_range', $this->timeRange)
            ->where('start_date', $this->startDate)
            ->orderByDesc('win_rate')
            ->orderByDesc('champion_rate')
            ->orderByDesc('win_count')
            ->get()
            ->each(function (RankReportHistory $rankReport) use (&$counter) {
                $rankReport->update([
                    'rank' => ++$counter
                ]);
            });
    }
}
