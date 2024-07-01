<?php


namespace App\Http\Controllers\Post;


use App\Enums\RankReportTimeRange;
use App\Helper\AccessTokenService;
use App\Http\Controllers\Controller;
use App\Http\Resources\Rank\RankReportHistoryResource;
use App\Models\Element;
use App\Models\Game;
use App\Models\GameRoom;
use App\Models\Post;
use App\Models\RankReport;
use App\Services\GameService;
use App\Services\RankService;
use App\Services\PostService;
use Illuminate\Http\Request;
use Illuminate\Pagination\LengthAwarePaginator;

class GameController extends Controller
{

    protected $gameService;
    protected $rankService;
    protected $postService;
    public function __construct(GameService $gameService, RankService $rankService, PostService $postService)
    {
        $this->gameService = $gameService;
        $this->rankService = $rankService;
        $this->postService = $postService;
    }

    public function show(Request $request)
    {
        $serial = $request->route('post');
        $post = $this->getPostOrFail($request);
        $element = $this->getElementForOG($post);
        $requiredPassword = $post->isPasswordRequired() && !AccessTokenService::verifyPostAccessToken($post);
        if (!$element) {
            return redirect()->back()->with('error', __('No element found in the post.'));
        }

        return $this->gameView($post, $element, $requiredPassword);
    }

    public function rank(Request $request)
    {
        $post = $this->getPostOrFail($request);

        // we first check if user has access to the post
        $embed = $request->session()->pull('rank-embed', false);

        if ($post->isPasswordRequired()) {
            if ($embed && $post->user_id == optional($request->user())->id) {
                // if user is the owner of the post and trying to embed the post
                // we allow the user to access the post
            } else if (AccessTokenService::verifyPostAccessToken($post) === false) {
                return redirect()->route('game.rank-access', ['post' => $post->serial]);
            }
        }elseif($post->user_id !== optional($request->user())->id){
            // if post is not password protected and user is not the owner of the post
            // we disable the embed option
            $embed = false;
        }

        $gameResult = $this->getGameResult($request);
        $reports = $this->rankService->getRankReports($post, 10);
        $myChampionHistories = $this->getChampionRankReportHistoryByGameResult($post, $gameResult, 365);

        return view('game.rank', [
            'serial' => $post->serial,
            'post' => $post,
            'ogElement' => $this->getElementForOG($post),
            'reports' => $reports,
            'gameResult' => $gameResult?->toResponse($request)->getData(),
            'champion_histories' => [
                'my' => $myChampionHistories
            ],
            'shared' => $request->query('s') ? true : false,
            'embed' => $embed
        ]);
    }

    public function accessRank(Request $request)
    {
        $post = $this->getPostOrFail($request);

        return view('game.rank-access', [
            'serial' => $post->serial,
        ]);
    }

    public function rankEmbed(Request $request)
    {
        // put data in session
        $request->session()->put('rank-embed', true);
        return $this->rank($request);
    }

    public function joinRoom(Request $request)
    {
        $gameRoom = GameRoom::where('serial', $request->route('gameRoom'))->first();
        if (!$gameRoom) {
            return redirect()->route('home')->with('error', __('Game not found.'));
        }
        $post = $gameRoom->game->post;
        $element = $this->getElementForOG($post);

        if (!$element) {
            return redirect()->back()->with('error', __('No element found in the post.'));
        }

        return $this->gameView($post, $element, false, $gameRoom);
    }

    protected function gameView(Post $post, $element, $requiredPassword, $gameRoom = null)
    {
        return view('game.show', [
            'serial' => $post->serial,
            'post' => $post,
            'gameRoom' => $gameRoom,
            'element' => $element,
            'requiredPassword' => $requiredPassword,
        ]);
    }

    protected function getElementForOG(Post $post, $limit = 10): ?Element
    {
        $reports = $this->rankService->getRankReports($post, 1)->items();
        if (count($reports) > 0) {
            return $reports[0]->element;
        }
        return $post->elements()->first();
    }

    protected function getPostOrFail(Request $request): Post
    {
        $serial = $request->route('post');
        $post = $this->postService->getPost($serial);
        abort_if(!$post, 404);
        $this->abortPrivatePost($post, $request);
        return $post;
    }

    protected function getGameResult(Request $request)
    {
        // g is for game, which user played complete game
        // s is for share, which user shared the game result
        $gameSerial = $request->query('g') ?? $request->query('s');
        $game = Game::where('serial', $gameSerial)->first();
        $gameResult = null;
        if ($game && $this->gameService->isGameComplete($game)) {
            $gameResult = $this->gameService->getGameResult($request, $game);
        }
        return $gameResult;
    }

    protected function getChampionRankReportHistoryByGameResult(Post $post, $gameResult, $limit = 10, $page = null)
    {
        if($gameResult) {
            $element = $gameResult->additional['winner'];
        }else{
            return null;
        }

        $championRankReportHistories = $this->rankService->getRankReportHistoryByElement(
            $post,
            $element,
            RankReportTimeRange::ALL,
            $limit,
            $page);
        return RankReportHistoryResource::collection($championRankReportHistories)
            ->toResponse(request())
            ->getData();

    }

    protected function getChampionRankReportHistory($gameResult, $limit = 10, $page = null)
    {
        if ($gameResult && isset($gameResult[0])) {
            $championRankReportHistories = $this->rankService->getRankReportHistoryByRankReport($gameResult[0], RankReportTimeRange::ALL, $limit, $page);
            return RankReportHistoryResource::collection($championRankReportHistories)
                ->toResponse(request())
                ->getData();
        }

        return null;
    }

    protected function abortPrivatePost(Post $post, Request $request)
    {
        if ($post->isPrivate() && $post->user_id !== optional($request->user())->id) {
            abort(403);
        }
    }

}
