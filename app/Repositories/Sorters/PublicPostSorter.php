<?php


namespace App\Repositories\Sorters;


use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Models\PostTrend;
use Illuminate\Database\Eloquent\Builder;

class PublicPostSorter
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
        if ($timeRange == TrendTimeRange::MONTH) {
            $query->orderBy('month_position');
        } else if ($timeRange == TrendTimeRange::WEEK) {
            $query->orderBy('week_position');
        } else if ($timeRange == TrendTimeRange::TODAY) {
            $query->orderBy('day_position');
        } else {
            $query->orderBy('week_position');
        }
        return $query;
    }

    public static function new(Builder $query)
    {
        $query->orderBy('new_position');
        return $query;
    }
}
