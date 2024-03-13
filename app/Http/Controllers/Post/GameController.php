<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Http\Resources\Game\GameResultResource;
use App\Http\Resources\Rank\PostRankResource;
use App\Models\Game;
use App\Models\Post;
use App\Services\GameService;
use App\Services\RankService;
use App\Services\PostService;
use Illuminate\Http\Request;

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
        $post = $this->postService->getPost($serial);
        $element = $this->getElementForOG($post);

        return view('game.show', [
            'serial' => $serial,
            'post' => $post,
            'element' => $element
        ]);
    }

    public function rank(Request $request)
    {
        $serial = $request->route('post');
        $tab = $request->query('tab');
        $gameSerial = $request->query('g');
        $game = Game::where('serial', $gameSerial)->first();
        $post = $this->postService->getPost($serial);
        $element = $this->getElementForOG($post);
        $reports = $this->rankService->getRankReports($post, 10);

        $gameResult = null;
        if ($game && $this->gameService->isGameComplete($game)) {
            $rounds = $game->game_1v1_rounds()
                ->orderBy('remain_elements')
                ->take(9)
                ->get();

            $winner = $this->gameService->getWinner($game);

            $gameResult = GameResultResource::collection($rounds)
                ->additional([
                    'winner' => $winner,
                    'winner_rank' => $this->rankService->getRankPosition($game->post, $winner)
                ])
                ->toResponse($request)->getData();
        }

        return view('game.rank', [
            'serial' => $serial,
            'post' => $post,
            'element' => $element,
            'reports' => $reports,
            'gameResult' => $gameResult
        ]);
    }

    protected function getElementForOG(Post $post, $limit = 10)
    {
        $reports = $this->rankService->getRankReports($post, 1)->items();
        if (count($reports) > 0) {
            return $reports[0]->element;
        }
        return $post->elements()->first();
    }

}
