<?php


namespace App\ScheduleExecutor;

use App\Models\Element;
use App\Services\ImageThumbnailService;
use App\Enums\ElementType;
use App\Jobs\RemoveOutdateRankHistory;
use App\Models\Post;
use Storage;

class RankReportScheduleExecutor
{
    public function removeOutdateRankReportHistory()
    {
        Post::withTrashed()->setEagerLoads([])->chunkById(300, function ($posts){
            foreach ($posts as $post) {
                RemoveOutdateRankHistory::dispatch($post);
            }
        });
    }
}
