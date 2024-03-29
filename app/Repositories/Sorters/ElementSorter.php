<?php


namespace App\Repositories\Sorters;

use Illuminate\Database\Eloquent\Builder;

class ElementSorter
{
    const ID = 'id';

    public static function apply(Builder $query, $sortBy, $dir)
    {
        if ($sortBy === self::ID){
            $query->orderBy('id', $dir);

        }

        return $query;
    }
}
