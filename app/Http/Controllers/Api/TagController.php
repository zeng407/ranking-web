<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use App\Models\Tag;
use Illuminate\Http\Request;
use App\Services\TagService;

class TagController extends Controller
{
    protected $tagService;
    protected $postService;

    public function __construct(TagService $tagService)
    {
        $this->tagService = $tagService;
    }

    public function index(Request $request)
    {
        $tags = CacheService::rememberHotTags();

        return response()->json($tags);
    }
}
