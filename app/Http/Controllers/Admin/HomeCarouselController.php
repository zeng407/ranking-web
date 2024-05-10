<?php

namespace App\Http\Controllers\Admin;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Services\PostService;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;

class HomeCarouselController extends Controller
{
    public function index()
    {
        return view('admin.home-carousel.index');
    }

    
}
