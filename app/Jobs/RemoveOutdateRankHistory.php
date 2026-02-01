<?php

namespace App\Jobs;

use App\Models\Post;
use App\Models\RankReportHistory;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Facades\DB;

class RemoveOutdateRankHistory implements ShouldQueue, ShouldBeUnique
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected Post $post;

    public function __construct(Post $post)
    {
        $this->post = $post;
        $this->onQueue('rank_report_history');
    }

    public function handle()
    {
        $idsToDelete = RankReportHistory::where('post_id', $this->post->id)
            ->where('start_date', '<', now()->subDays(93))
            ->limit(1000)
            ->pluck('id');

        if ($idsToDelete->isEmpty()) {
            return;
        }

        RankReportHistory::whereIn('id', $idsToDelete)->forceDelete();
    }

    public function uniqueId()
    {
        return $this->post->serial;
    }
}
