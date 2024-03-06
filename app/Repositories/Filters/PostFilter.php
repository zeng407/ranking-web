<?php


namespace App\Repositories\Filters;


use App\Enums\PostAccessPolicy;
use App\Models\Post;
use Illuminate\Database\Eloquent\Builder;

class PostFilter
{
    const KEYWORD_LIKE = 'keyword_like';
    const ELEMENTS_COUNT_GTE = 'elements_count_gte';
    const PUBLIC = 'public';

    public static function apply(Builder $query, array $condition)
    {
        if (isset($condition[self::KEYWORD_LIKE])) {
            $query = self::keyword_like($query, $condition[self::KEYWORD_LIKE]);
        }

        if (isset($condition[self::ELEMENTS_COUNT_GTE])) {
            $query = self::elements_count_gte($query, $condition[self::ELEMENTS_COUNT_GTE]);
        }

        if (isset($condition[self::PUBLIC])) {
            $query = self::public($query, $condition[self::PUBLIC]);
        }

        return $query;
    }

    public static function keyword_like(Builder $query, $value)
    {
        if (is_string($value)) {
            $value = explode(' ', trim($value));
        }
        $value = array_filter($value, fn($v) => !is_null($v) && '' !== $v);

        if (count($value) > 0) {
            foreach ($value as $keyword) {
                $query->where(
                    function ($query) use ($keyword) {
                        $query->orWhere('title', 'like', "%$keyword%")
                            ->orWhere('description', 'like', "%$keyword%")
                            ->orWhereHas('tags', function ($query) use ($keyword) {
                                $query->where('name', 'like', "%$keyword%")
                                    ->orWhere('name', 'like', "%" . str_replace('#', '', $keyword) . "%");
                            });
                    }
                );
            }
        }
        return $query;
    }

    public static function elements_count_gte(Builder $query, $value)
    {
        return $query->whereHas('elements', null, '>=', $value);
    }

    public static function public(Builder $query, $value)
    {
        return $query->whereHas('post_policy', function ($query) {
            $query->where('access_policy', PostAccessPolicy::PUBLIC );
        });
        //        return (new Post)->scopePublic($query);
    }
}
