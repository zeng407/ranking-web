<?php


namespace App\Repositories\Filters;


use Illuminate\Database\Eloquent\Builder;

class ElementFilter
{
    const TITLE_LIKE = 'title_like';
    const POST_ID = 'post_id';

    public static function apply(Builder $query, array $condition)
    {
        if(isset($condition[self::POST_ID])){
            $query = self::post_id($query, $condition[self::POST_ID]);
        }

        if(isset($condition[self::TITLE_LIKE])){
            $query = self::title_like($query, $condition[self::TITLE_LIKE]);
        }

        return $query;
    }

    public static function post_id(Builder $query, $value)
    {
        return $query->whereHas('posts', function($query)use($value){
            $query->where('posts.id', $value);
        });
    }

    public static function title_like(Builder $query, $value)
    {
        return $query->where('title', 'like', "%$value%");
    }
}
