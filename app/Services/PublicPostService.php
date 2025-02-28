<?php


namespace App\Services;


use App\Repositories\PublicPostRepository;
use App\Services\Traits\HasRepository;

class PublicPostService
{
    use HasRepository;

    protected $repo;

    public function __construct(PublicPostRepository $repo)
    {
        $this->repo = $repo;
    }

}
