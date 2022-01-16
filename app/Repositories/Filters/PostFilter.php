<?php


namespace App\Repositories\Filters;


use App\Models\Post;
use Illuminate\Database\Eloquent\Builder;

class PostFilter
{
    const ANY_LIKE = 'any_like';
    const ELEMENTS_COUNT_GTE = 'elements_count_gte';
    const PUBLIC = 'public';

    public static function apply(Builder $query, array $condition)
    {
        if(isset($condition[self::ANY_LIKE])){
            $query = self::any_like($query, $condition[self::ANY_LIKE]);
        }

        if(isset($condition[self::ELEMENTS_COUNT_GTE])){
            $query = self::elements_count_gte($query, $condition[self::ELEMENTS_COUNT_GTE]);
        }

        if(isset($condition[self::PUBLIC])){
            $query = self::public($query, $condition[self::PUBLIC]);
        }

        return $query;
    }

    public static function any_like(Builder $query, $value)
    {
        return $query->orWhere('title', 'like', "%$value%")
            ->orWhere('description', 'like', "%$value%");
    }

    public static function elements_count_gte(Builder $query, $value)
    {
        return $query->whereHas('elements', null, '>=', $value);
    }

    public static function public(Builder $query, $value)
    {
        return (new Post)->scopePublic($query);
    }
}
