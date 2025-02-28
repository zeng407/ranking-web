<?php


namespace App\Repositories\Filters;


use App\Enums\PostAccessPolicy;
use App\Models\Post;
use Illuminate\Database\Eloquent\Builder;

class PublicPostFilter
{
    const KEYWORD_LIKE = 'keyword_like';
    const ELEMENTS_COUNT_GTE = 'elements_count_gte';
    const PUBLIC = 'public';

    public static function apply(Builder $query, array $condition)
    {
        if (isset($condition[self::KEYWORD_LIKE])) {
            $query = self::keyword_like($query, $condition[self::KEYWORD_LIKE]);
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

            $value = array_slice($value, 0, 10);
            foreach ($value as $keyword) {

                $query->where(
                    function ($query) use ($keyword) {
                        $keyword = str_replace('#', '', $keyword);
                        $query->orWhere('title', 'like', "%$keyword%")
                            ->orWhere('description', 'like', "%$keyword%")
                            ->orwhere('tags', 'like', "%$keyword%");
                    }
                );
            }
        }
        return $query;
    }
}
