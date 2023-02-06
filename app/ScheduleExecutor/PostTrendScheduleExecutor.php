<?php


namespace App\ScheduleExecutor;


use App\Enums\RankType;
use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
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

        $query = Game::whereRaw('games.post_id = posts.id')
            ->selectRaw('count(*)')
            ->where(function ($query) use ($startDate) {
                if ($startDate) {
                    $query->where('created_at', '>=', $startDate);
                }
            });

        Post::orderByRaw("( {$query->toSql()} ) desc", $query->getBindings())
            ->each(function (Post $post, $count) use ($range, $startDate) {
                $post->post_trends()->updateOrCreate([
                    'trend_type' => TrendType::HOT,
                    'time_range' => $range,
                    'start_date' => $startDate
                ], [
                    'trend_type' => TrendType::HOT,
                    'time_range' => $range,
                    'position' => $count + 1,
                    'start_date' => $startDate
                ]);
            });
    }
}
