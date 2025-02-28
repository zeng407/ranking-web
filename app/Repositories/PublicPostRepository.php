<?php


namespace App\Repositories;


use App\Models\PublicPost;
use App\Repositories\Filters\PublicPostFilter;
use App\Repositories\Sorters\PublicPostSorter;
use Illuminate\Database\Eloquent\Builder;

class PublicPostRepository
{
    public function filter(array $conditions)
    {
        $query = PublicPost::getModel()->newQuery();

        $query = PublicPostFilter::apply($query, $conditions);

        return $query;
    }

    public function sorter(Builder $query, $sortBy, $dir)
    {
        return PublicPostSorter::apply($query, $sortBy, $dir);
    }

}
