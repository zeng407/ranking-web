<?php

namespace App\Jobs;

use App\Enums\TrendType;
use App\Models\Post;
use App\Models\PostTrend;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class UpdatePostTrendsPosition implements ShouldQueue, ShouldBeUnique
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    protected null|string $startDate;
    protected string $range;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(?string $startDate, string $range)
    {
        $this->startDate = $startDate;
        $this->range = $range;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        $count = 0;
        \DB::table('posts')
            ->join('post_statistics', 'posts.id', '=', 'post_statistics.post_id')
            ->where(function ($query) {
                if($this->startDate){
                    $query->where('post_statistics.start_date', $this->startDate);
                }
            })
            ->where('post_statistics.time_range', $this->range)
            ->orderBy('post_statistics.play_count', 'desc')
            ->orderBy('posts.id', 'desc')
            ->select('posts.id')
            ->each(function ($post) use (&$count) {
                $count++;
                logger('UpdatePostTrendsPosition job fired postId: ' . $post->id . ' count: ' . $count);
                PostTrend::updateOrCreate([
                        'post_id' => $post->id,
                        'trend_type' => TrendType::HOT,
                        'time_range' => $this->range,
                        'start_date' => $this->startDate
                    ], [
                        'post_id' => $post->id,
                        'trend_type' => TrendType::HOT,
                        'time_range' => $this->range,
                        'start_date' => $this->startDate,
                        'position' => $count,
                    ]);
            });
    }

    public function uniqueId()
    {
        return $this->startDate . $this->range;
    }
}
