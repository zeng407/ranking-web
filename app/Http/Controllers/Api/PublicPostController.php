<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;

use App\Http\Resources\PublicPostResource;
use App\Models\Post;

class PublicPostController extends Controller
{
    public function index()
    {
        $query = Post::public()->whereHas('elements', null, '>=', 8);

        return PublicPostResource::collection($query->paginate());
    }
}
