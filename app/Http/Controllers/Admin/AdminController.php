<?php

namespace App\Http\Controllers\Admin;

use App\Enums\PostAccessPolicy;
use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Models\Tag;
use App\Services\PostService;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;
use App\Models\Element;

class AdminController extends Controller
{
    protected PostService $postService;

    public function __construct(PostService $postService)
    {
        $this->postService = $postService;
    }
    
    public function dashboard()
    {
        return view('admin.dashboard');
    }

}
