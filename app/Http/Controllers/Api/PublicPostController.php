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
        $query = Post::whereHas('post_policy', function($query){
            $query->where('access_policy', PostAccessPolicy::PUBLIC);
        });

        return PublicPostResource::collection($query->paginate());
    }
}
