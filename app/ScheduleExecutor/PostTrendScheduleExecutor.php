<?php


namespace App\ScheduleExecutor;


use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
use App\Jobs\UpdatePostTrendsPosition;
use App\Models\Post;

class PostTrendScheduleExecutor
{
    public function createAllPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::ALL);
    }

    public function createYearPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::YEAR);
    }

    public function createMonthPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::MONTH);
    }

    public function createWeekPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::WEEK);
    }

    public function createTodayPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::TODAY);
    }

    protected function createHotTrendPost(string $range)
    {
        logger()->info("Start creating post trend for range: $range");
        $startDate = null;
        switch ($range) {
            case TrendTimeRange::ALL:
                $startDate = null;
                break;
            case TrendTimeRange::YEAR:
                $startDate = today()->startOfYear()->toDateString();
                break;
            case TrendTimeRange::MONTH:
                $startDate = today()->startOfMonth()->toDateString();
                break;
            case TrendTimeRange::WEEK:
                $startDate = today()->startOfWeek()->toDateString();
                break;
            case TrendTimeRange::TODAY:
                $startDate = today()->toDateString();
                break;
        }

        Post::withCount(['games' => function ($query) use ($startDate) {
                if ($startDate){
                    $query->where('created_at', '>=', $startDate);
                }
                $query->where('vote_count', '>=', 4);
            }])
            ->setEagerLoads([])
            ->eachById(function (Post $post) use ($startDate, $range) {
                $date = $startDate ?: $post->created_at->toDateString();
                try {
                    $post->post_statistics()->updateOrCreate([
                        'start_date' => $date,
                        'time_range' => $range
                    ], [
                        'start_date' => $date,
                        'time_range' => $range,
                        'play_count' => $post->games_count
                    ]);
                } catch (\Illuminate\Database\QueryException $e) {
                    $errorCode = $e->errorInfo[1] ?? null;
                    // 1062 是 MySQL 的 Duplicate entry 錯誤碼
                    if ($errorCode == 1062 || $e->getCode() == '23000') {
                        \Log::warning("Duplicate entry skipped for post_statistics", [
                            'post_id' => $post->id,
                            'start_date' => $date,
                            'time_range' => $range,
                            'play_count' => $post->games_count
                        ]);
                    } else {
                        // 若為其他錯誤則拋出
                        throw $e;
                    }
                }
            });

        UpdatePostTrendsPosition::dispatch($startDate, $range);
    }
}
