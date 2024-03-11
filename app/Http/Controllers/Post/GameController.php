<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
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
        $post = $this->postService->getPost($serial);
        $element = $this->getElementForOG($post);
        return view('game.rank', [
            'serial' => $serial,
            'post' => $post,
            'element' => $element
        ]);
    }

    protected function getElementForOG(Post $post, $limit = 10)
    {
        $reports = $this->rankService->getRankReports($post, 1)->items();
        if(count($reports) > 0){
            return $reports[0]->element;
        }
        return $post->elements()->first();
    }

}
