<?php

namespace App\Http\Controllers\Api;

use App\Enums\RankReportTimeRange;
use App\Http\Controllers\Controller;
use App\Http\Resources\Rank\RankReportHistoryResource;
use App\Http\Resources\Rank\RankReportResource;
use App\Models\Element;
use App\Models\Post;
use App\Services\RankService;
use Illuminate\Http\Request;


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
            'time' => ['required', 'array', 'in:'. implode(',',RankReportTimeRange::toArray())],
        ]);
        $post = $this->getPost($request->post_serial);
        /** @see \App\Policies\PostPolicy::readRank() */
        $this->authorize('read-rank', $post);

        $result = [];
        $result['current'] = RankReportResource::make($this->rankService->getRankReportByElement($post, $this->getElement($request->element_id)));
        $element = $this->getElement($request->element_id);
        foreach ((array)$request->time as $time){
            $time = RankReportTimeRange::from($time);
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
            $result[$time->value] = RankReportHistoryResource::collection($ranks);
        }

        return $result;
    }

    public function searchRank(Request $request)
    {
        $request->validate([
            'post_serial' => ['required', 'string', 'max:255'],
            'keyword' => ['required', 'string', 'max:255'],
        ]);
        $post = $this->getPost($request->post_serial);
        /** @see \App\Policies\PostPolicy::readRank() */
        $this->authorize('read-rank', $post);

        $result = RankReportResource::collection($this->rankService->getRanksByElementTitle($post, $request->keyword));

        return $result;
    }

    protected function getPost($postSerial)
    {
        return Post::where('serial', $postSerial)->firstOrFail();
    }

    protected function getElement($elementId)
    {
        return Element::findOrFail($elementId);
    }

}
