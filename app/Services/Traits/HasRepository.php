<?php

namespace App\Services\Traits;

trait HasRepository
{
    public function getList(array $conditions, array $sorter = [], array $paginationOptions = [], array $with = [])
    {
        //trim null or empty
        $conditions = array_filter($conditions, fn($value) => !is_null($value) && $value !== '');
        $sorter = array_filter($sorter, fn($value) => !is_null($value) && $value !== '');

        $query = $this->repo->filter($conditions);
        if($with){
            $query = $query->with($with);
        }

        if ($sortBy = data_get($sorter, 'sort_by')) {
            $dir = data_get($sorter, 'sort_dir') === 'desc' ? 'desc' : 'asc';
            $query = $this->repo->sorter($query, $sortBy, $dir);
        }

        $perPage = data_get($paginationOptions, 'per_page', 15);

        return $query->paginate($perPage);
    }

}