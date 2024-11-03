<?php


namespace App\ScheduleExecutor;


use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
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
            ->orderBy('games_count', 'desc')
            ->orderBy('posts.id', 'desc')
            ->eachById(function (Post $post) use ($startDate, $range) {
                $date = $startDate ?: $post->created_at->toDateString();
                $post->post_statistics()->updateOrCreate([
                    'start_date' => $date,
                    'time_range' => $range
                ], [
                    'start_date' => $date,
                    'time_range' => $range,
                    'play_count' => $post->games_count
                ]);
            });

        $count = 0;
        Post::join('post_statistics', 'posts.id', '=', 'post_statistics.post_id')
            ->where(function ($query) use ($startDate) {
                if($startDate){
                    $query->where('post_statistics.start_date', $startDate);
                }
            })
            ->where('post_statistics.time_range', $range)
            ->orderBy('post_statistics.play_count', 'desc')
            ->orderBy('posts.id', 'desc')
            ->selectRaw('posts.*')
            ->eachById(function (Post $post) use ($range, $startDate, &$count) {
                $count++;
                $post->post_trends()->updateOrCreate([
                    'trend_type' => TrendType::HOT,
                    'time_range' => $range,
                    'start_date' => $startDate
                ], [
                    'trend_type' => TrendType::HOT,
                    'time_range' => $range,
                    'position' => $count,
                    'start_date' => $startDate
                ]);
            });
    }
}
