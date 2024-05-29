<?php

namespace App\Http\Controllers\Api;

use App\Enums\PostAccessPolicy;
use App\Enums\RankReportTimeRange;
use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use App\Http\Resources\Comment\CommentResource;
use App\Http\Resources\Game\ChampionResource;
use App\Http\Resources\PublicPostResource;
use App\Http\Resources\Rank\RankReportHistoryResource;
use App\Http\Resources\Rank\WeeklyRankResource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\UserGameResult;
use App\Services\Builders\CommentBuilder;
use App\Services\PostService;
use App\Services\RankService;
use Illuminate\Http\Request;
use RateLimiter;
use App\Models\Comment;


class RankController extends Controller
{
    protected RankService $rankService;

    public function __construct(RankService $rankService)
    {
        $this->rankService = $rankService;
    }

    public function getRank(Request $request)
    {
        $request->validate([
            'post_serial' => ['required', 'string', 'max:255'],
            'element_id' => ['required', 'integer'],
            'time' => ['required', 'string', 'in:'. implode(',',RankReportTimeRange::toArray())],
        ]);
        $post = $this->getPost($request->post_serial);
        $element = $this->getElement($request->element_id);
        $time = RankReportTimeRange::from($request->time);
        if($time == RankReportTimeRange::YEAR){
            $limit = 10;
        }elseif($time == RankReportTimeRange::WEEK){
            $limit = 52;
        }elseif($time == RankReportTimeRange::MONTH){
            $limit = 12;
        }elseif($time == RankReportTimeRange::ALL){
            $limit = 365;
        }else{
            $limit = 100;
        }
        $ranks = $this->rankService->getRankReportHistoryByElement($post, $element, $time, $limit, 1);

        return RankReportHistoryResource::collection($ranks);
    }

    protected function getPost($postSerial)
    {
        return  Post::where('serial', $postSerial)->firstOrFail();
    }

    protected function getElement($elementId)
    {
        return Element::findOrFail($elementId);
    }

}
