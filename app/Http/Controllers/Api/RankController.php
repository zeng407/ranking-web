<?php

namespace App\Http\Controllers\Api;

use App\Enums\RankType;
use App\Http\Controllers\Controller;
use App\Http\Resources\Rank\PostRankResource;
use App\Http\Resources\Rank\RankReportResource;
use App\Models\Post;
use App\Models\RankReport;
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

    public function index($serial)
    {
        $post = Post::where('serial',$serial)->firstOrFail();

        return PostRankResource::make($post);
    }

    public function update(Request $request, $serial)
    {
        $post = Post::where('serial', $serial)->firstOrFail();

        if($gameSerial = $request->input('g')){
            $game = $post->games()->where('serial', $gameSerial)->first();
            if($game && $this->gameService->isGameComplete($game)){
                $this->rankService->createRankReport($post);
            }
        }

        return response();
    }

    public function report($serial)
    {
        $post = Post::where('serial',$serial)->firstOrFail();

        $report = RankReport::where('post_id', $post->id)
            ->orderByDesc('final_win_rate')
            ->orderByDesc('win_rate')
            ->paginate(10);

        return RankReportResource::collection($report);
    }
}
