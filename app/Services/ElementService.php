<?php


namespace App\Services;


use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Repositories\ElementRepository;

class ElementService
{
    protected $repo;

    public function __construct(ElementRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getLists(array $conditions, array $paginationOptions = [])
    {
        $query = $this->repo->filter($conditions);

        $perPage = 15;

        if(isset($paginationOptions['per_page'])){
            $perPage = $paginationOptions['per_page'];
        }

        return $query->paginate($perPage);
    }


}
