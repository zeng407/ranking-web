<?php


namespace App\Repositories\Sorters;


use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Models\PostTrend;
use Illuminate\Database\Eloquent\Builder;

class PostSorter
{
    const HOT_ALL = 'hot_all';
    const HOT_YEAR = 'hot_year';
    const HOT_MONTH = 'hot_month';
    const HOT_WEEK = 'hot_week';
    const HOT_DAY = 'hot_day';
    const NEW = 'new';

    public static function apply(Builder $query, $sortBy, $dir)
    {
        if ($sortBy === self::HOT_ALL) {
            $query = self::hot($query, TrendTimeRange::ALL);
        } else if ($sortBy === self::HOT_YEAR) {
            $query = self::hot($query, TrendTimeRange::YEAR);
        } else if ($sortBy === self::HOT_MONTH) {
            $query = self::hot($query, TrendTimeRange::MONTH);
        } else if ($sortBy === self::HOT_WEEK) {
            $query = self::hot($query, TrendTimeRange::WEEK);
        } else if ($sortBy === self::HOT_DAY) {
            $query = self::hot($query, TrendTimeRange::TODAY);
        } else if ($sortBy === self::NEW) {
            $query = self::new($query);
        }

        return $query;
    }

    public static function hot(Builder $query, $timeRange)
    {
        $orderRaw = PostTrend::whereRaw('post_trends.post_id = posts.id')
            ->select('position')
            ->whereRaw('post_trends.post_id = posts.id')
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', $timeRange)
            ->orderByDesc('id')
            ->limit(1);

        $query->orderByRaw(" ifnull( ({$orderRaw->toSql()}) , 999999) ", $orderRaw->getBindings());

        return $query;
    }

    public static function new(Builder $query)
    {
        $query->orderByDesc('created_at');
        return $query;
    }
}
