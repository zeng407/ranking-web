<?php

namespace App\Jobs;

use App\Models\Post;
use App\Services\RankService;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;


class UpdateRankReport implements ShouldQueue, ShouldBeUnique
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
        logger('UpdateRankReport job fired');
        $rankService->createRankReports($this->post);
    }


    public function uniqueId()
    {
        return $this->post->serial;
    }
}
