<?php


namespace App\Repositories;


use App\Models\Element;
use App\Repositories\Filters\ElementFilter;

class ElementRepository
{
    public function filter(array $conditions)
    {
        $query = Element::getModel()->newQuery();

        $query = ElementFilter::apply($query, $conditions);

        return $query;
    }
}
