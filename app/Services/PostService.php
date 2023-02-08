<?php


namespace App\Services;


use App\Repositories\PostRepository;

class PostService
{
    protected $repo;

    public function __construct(PostRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getLists(array $conditions, array $sorter = [] , array $paginationOptions = [])
    {
        //trim null or empty
        $conditions = array_filter($conditions, fn($value) => !is_null($value) && $value !== '');
        $sorter = array_filter($sorter, fn($value) => !is_null($value) && $value !== '');

        $query = $this->repo->filter($conditions);
        \Log::info($query->toSql());
        if($sortBy = data_get($sorter, 'sort_by')){
            $dir = data_get($sorter, 'sort_dir') === 'desc' ? 'desc' : 'asc';
            $query = $this->repo->sorter($query, $sortBy, $dir);
        }

        $perPage = data_get($paginationOptions, 'per_page', 15);

        return $query->paginate($perPage);
    }


}
