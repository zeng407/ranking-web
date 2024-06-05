<?php


namespace App\ScheduleExecutor;


use App\Enums\RankType;
use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
use App\Models\Element;
use App\Models\Game;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Services\RankService;
use DB;

class PostTrendScheduleExecutor
{
    public function createPostTrends()
    {
        $this->createHotTrendPost(TrendTimeRange::ALL);
        $this->createHotTrendPost(TrendTimeRange::YEAR);
        $this->createHotTrendPost(TrendTimeRange::MONTH);
        $this->createHotTrendPost(TrendTimeRange::WEEK);
        $this->createHotTrendPost(TrendTimeRange::TODAY);
        CacheService::rememberPostUpdatedTimestamp(true);
    }

    protected function createHotTrendPost($range)
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
            }])
            ->orderBy('games_count', 'desc')
            ->orderBy('posts.id', 'desc')
            ->eachById(function (Post $post) use ($startDate) {
                $date = $startDate ?: $post->created_at->toDateString();
                $post->post_statistics()->updateOrCreate([
                    'start_date' => $date
                ], [
                    'start_date' => $date,
                    'play_count' => $post->games_count
                ]);
            });

        $count = 0;
        Post::join('post_statistics', 'posts.id', '=', 'post_statistics.post_id')
            ->where('post_statistics.start_date', $startDate)
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
