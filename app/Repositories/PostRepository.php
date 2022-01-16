<?php


namespace App\Repositories;


use App\Models\Post;
use App\Repositories\Filters\PostFilter;
use App\Repositories\Sorters\PostSorter;
use Illuminate\Database\Eloquent\Builder;

class PostRepository
{
    public function filter(array $conditions)
    {
        $query = Post::getModel()->newQuery();

        $query = PostFilter::apply($query, $conditions);

        return $query;
    }

    public function sorter(Builder $query, $sortBy, $dir)
    {
        return PostSorter::apply($query, $sortBy, $dir);
    }

}
