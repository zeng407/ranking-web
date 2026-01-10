<?php
namespace App\Jobs;

use App\Enums\RankReportTimeRange;
use App\Helper\CacheService;
use App\Helper\Locker;
use App\Models\RankReport;
use App\Services\RankService;
use Cache;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Contracts\Queue\ShouldBeUnique;

class CreateRankReportHistory implements ShouldQueue, ShouldBeUnique
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected RankReport $rankReport;
    protected $refresh;
    protected $startAt;


    public function __construct(RankReport $rankReport, $refresh = false, $start = null)
    {
        $this->rankReport = $rankReport;
        $this->refresh = $refresh;
        $this->startAt = $start;
        $this->onQueue('rank_report_history');
    }

    /**
     * Execute the job.
     *
     * @param RankService $rankService
     * @return void
     */
    public function handle(RankService $rankService)
    {
        // logger('CreateRankReportHistory job fired for post id: ' . $this->rankReport->post_id);
        $rankService->createRankReportHistory($this->rankReport, RankReportTimeRange::ALL, $this->refresh, $this->startAt);
        $rankService->createRankReportHistory($this->rankReport, RankReportTimeRange::THOUSAND_VOTES, $this->refresh, $this->startAt);

        try{
            $locker = Locker::lockRankJob($this->rankReport->post_id);
            $locker->block(5);
            $count = CacheService::getRankHistoryJobCache($this->rankReport->post_id);
            $count = is_numeric($count) ? (int) $count : 0;
            // logger('lockRankJob', ['post_id' => $this->rankReport->post_id, 'count' => $count]);
            if ($count > 0) {
                $count--;
            }
            CacheService::putRankHistoryJobCache($this->rankReport->post_id, $count);
        }catch (\Exception $e){
            logger('lock error', ['post_id' => $this->rankReport->post_id, 'error' => $e->getMessage()]);
        } finally {
            if (isset($locker)) {
                $locker->release();
            }
        }

    }

    public function uniqueId()
    {
        return $this->rankReport->id;
    }
}
