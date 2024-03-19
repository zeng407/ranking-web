<?php


namespace App\ScheduleExecutor;


use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Services\RankService;

class RankScheduleExecutor
{
    protected $rankService;

    public function __construct(RankService $rankService)
    {
        $this->rankService = $rankService;
    }

    public function createRankReports()
    {
        Post::eachById(function (Post $post, $count) {
            logger("[createRankReport] post {$post->id} {$post->serial}");
            $this->rankService->createRankReport($post);
        });

    }
}
