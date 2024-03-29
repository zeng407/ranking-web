<?php


namespace App\Repositories;


use App\Models\Element;
use App\Repositories\Filters\ElementFilter;
use App\Repositories\Sorters\ElementSorter;
use Illuminate\Database\Eloquent\Builder;

class ElementRepository
{
    public function filter(array $conditions)
    {
        $query = Element::getModel()->newQuery();
        $query = ElementFilter::apply($query, $conditions);

        return $query;
    }

    public function sorter(Builder $query, $sortBy, $dir)
    {
        return ElementSorter::apply($query, $sortBy, $dir);
    }
}
