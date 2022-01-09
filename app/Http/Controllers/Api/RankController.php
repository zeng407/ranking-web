<?php

namespace App\Http\Controllers\Api;

use App\Enums\RankType;
use App\Http\Controllers\Controller;
use App\Http\Resources\Rank\PostRankResource;
use App\Http\Resources\Rank\RankReportResource;
use App\Models\Post;
use App\Models\RankReport;
use App\Policies\PostPolicy;
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;
use Illuminate\Pagination\LengthAwarePaginator;

class RankController extends Controller
{
    protected $gameService;
    protected $rankService;

    public function __construct(GameService $gameService, RankService $rankService)
    {
        $this->gameService = $gameService;
        $this->rankService = $rankService;
    }

    public function index(Post $post)
    {
        /**
         * @see PostPolicy::publicRead()
         */
        $this->authorize('public-read', $post);

        return PostRankResource::make($post);
    }

    public function report(Post $post)
    {
        /**
         * @see PostPolicy::publicRead()
         */
        $this->authorize('public-read', $post);

        $reports = $this->rankService->getRankReports($post, 10);

        return RankReportResource::collection($reports);
    }
}
