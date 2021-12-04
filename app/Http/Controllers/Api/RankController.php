<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Http\Resources\PostResource;
use App\Http\Resources\RankResource;
use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class RankController extends Controller
{
    public function __construct()
    {

    }

    public function show($serial)
    {
        $post = Post::where('serial',$serial)->firstOrFail();

        return RankResource::make($post);
    }
}
