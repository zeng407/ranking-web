<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Post;
use App\Services\GameService;
use App\Services\RankService;
use Illuminate\Http\Request;

class GameController extends Controller
{

    protected $gameService;
    protected $rankService;

    public function __construct(GameService $gameService, RankService $rankService)
    {
        $this->gameService = $gameService;
        $this->rankService = $rankService;
    }

    public function show(Request $request)
    {
        $serial = $request->route('post');
        return view('game.show', compact('serial'));
    }

    public function rank(Request $request)
    {
        $serial = $request->route('post');
        return view('game.rank', compact('serial'));
    }

}
