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
        $request->validate([
            'page' => 'nullable|numeric|min:1',
        ]);

        $start = microtime(true);
        $lastMark = $start;
        $timings = [];
        $mark = function (string $label) use (&$timings, &$lastMark) {
            $now = microtime(true);
            $timings[$label] = round(($now - $lastMark) * 1000, 2);
            $lastMark = $now;
        };

        $post = $this->getPostOrFail($request);
        $mark('get_post');

        // we first check if user has access to the post
        $embed = $request->session()->pull('rank-embed', false);
        $mark('session_pull');

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
        $mark('access_check');

        $gameResult = $this->getGameResult($request);
        $mark('get_game_result');

        $reports = $this->rankService->getRankReports($post, 10);
        $mark('get_rank_reports');

        $myChampionHistories = $this->getChampionRankReportHistoryByGameResult($post, $gameResult, 365);
        $mark('get_champion_histories');

        $totalMs = round((microtime(true) - $start) * 1000, 2);
        $sumMs = round(array_sum($timings), 2);
        $gapMs = round($totalMs - $sumMs, 2);

        logger('rank timings', [
            'post_id' => $post->id,
            'timings_ms' => $timings,
            'sum_ms' => $sumMs,
            'gap_ms' => $gapMs,
            'total_ms' => $totalMs,
        ]);

        return view('game.rank', [
            'serial' => $post->serial,
            'post' => $post,
            'ogElement' => $this->getElementForOG($post),
            'reports' => $reports,
            'gameResult' => $gameResult,
            'champion_histories' => [
                'my' => $myChampionHistories
            ],
            'shared' => $request->query('s') ? true : false,
            'embed' => $embed
        ]);
    }

    public function export(Request $request)
    {
        $post = $this->getPostOrFail($request);

        if ($post->isPasswordRequired()) {
            if (AccessTokenService::verifyPostAccessToken($post) === false) {
                return redirect()->route('game.rank-access', ['post' => $post->serial]);
            }
        }


        $gameResult = $this->getGameResult($request);

        return view('game.rank-export', [
            'serial' => $post->serial,
            'post' => $post,
            'ogElement' => $this->getElementForOG($post),
            'gameResult' => $gameResult,
            'embed' => false // Added embed variable as it seems used in the view layout
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
        $userLastGame = $this->gameService->getUserLastGame($post, request()->user());

        return view('game.show', [
            'serial' => $post->serial,
            'post' => $post,
            'userLastGame' => $userLastGame,
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

    protected function getGameResult(Request $request): array|null
    {
        // g is for game, which user played complete game
        // s is for share, which user shared the game result
        $gameSerial = $request->query('g') ?? $request->query('s');
        $game = Game::where('serial', $gameSerial)->first();
        $gameResult = null;
        if ($game && $this->gameService->isGameComplete($game)) {
            $gameResult = $this->gameService->getGameResult($game);
        }
        return $gameResult;
    }

    protected function getChampionRankReportHistoryByGameResult(Post $post, array|null $gameResult, $limit = 10, $page = null)
    {
        if($gameResult && isset($gameResult['winner'])){
            $element = $gameResult['winner'];
        }else{
            return null;
        }

        $championRankReportHistories = $this->rankService->getRankReportHistoryByElement(
            $post,
            $element['id'] ?? null,
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
