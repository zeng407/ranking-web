<?php


namespace App\Repositories\Sorters;

use Illuminate\Database\Eloquent\Builder;

class ElementSorter
{
    const ID = 'id';

    const RANK = 'rank';

    public static function apply(Builder $query, $sortBy, $dir)
    {
        if ($sortBy === self::ID){
            $query->orderBy('id', $dir);
        }elseif ($sortBy === self::RANK){
            $query->leftJoin(\DB::raw('rank_reports as _sort_rank_reports'), function($join){
                $join->on('elements.id', '=', '_sort_rank_reports.element_id');
            })->orderByRaw('isnull(_sort_rank_reports.rank)')
                ->orderBy('_sort_rank_reports.rank', $dir)
                ->orderBy('elements.id')
                ->select('elements.*');
        }

        return $query;
    }
}
