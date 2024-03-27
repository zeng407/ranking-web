<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Element;
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
        abort_if(!$post, 404);
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
        // g is for game, which user played complete game
        // s is for share, which user shared the game result
        $gameSerial = $request->query('g') ?? $request->query('s');
        $game = Game::where('serial', $gameSerial)->first();
        $post = $this->postService->getPost($serial);
        $reports = $this->rankService->getRankReports($post, 10);
        
        $gameResult = null;
        if ($game && $this->gameService->isGameComplete($game)) {
            $gameResult = $this->gameService->getGameResult($request, $game);
        }

        return view('game.rank', [
            'serial' => $serial,
            'post' => $post,
            'ogElement' => $this->getElementForOG($post),
            'reports' => $reports,
            'gameResult' => $gameResult,
            'shared' => $request->query('s') ? true : false
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

}
