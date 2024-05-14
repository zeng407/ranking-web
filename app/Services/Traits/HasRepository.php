<?php

namespace App\Services\Traits;

trait HasRepository
{
    protected $cursorPaginate = false;

    public function useCursorPaginate()
    {
        $this->cursorPaginate = true;
        return $this;
    }

    /**
     * @return \Illuminate\Contracts\Pagination\CursorPaginator|\Illuminate\Contracts\Pagination\LengthAwarePaginator
     */
    public function getList(array $conditions, array $sorter = [], array $paginationOptions = [], array $with = [])
    {
        //trim null or empty
        $conditions = array_filter($conditions, fn($value) => !is_null($value) && $value !== '');
        $sorter = array_filter($sorter, fn($value) => !is_null($value) && $value !== '');

        /** @var \Illuminate\Database\Eloquent\Builder $query */
        $query = $this->repo->filter($conditions);
        if($with){
            $query = $query->with($with);
        }

        if ($sortBy = data_get($sorter, 'sort_by')) {
            $dir = data_get($sorter, 'sort_dir') === 'desc' ? 'desc' : 'asc';
            /** @var \Illuminate\Database\Eloquent\Builder $query */
            $query = $this->repo->sorter($query, $sortBy, $dir);
        }

        $perPage = data_get($paginationOptions, 'per_page', 15);

        if($this->cursorPaginate){
            return $query->cursorPaginate($perPage);
        }else{
            return $query->paginate($perPage);
        }
    }

}