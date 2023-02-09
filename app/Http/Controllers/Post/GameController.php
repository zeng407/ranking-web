<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use App\Models\Game;
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

    public function show($serial)
    {
        $post = Post::where('serial', $serial)->first();
        return view('game.show', [
            'serial' => $serial,
            'post' => $post
        ]);
    }

    public function rank($serial)
    {
        $post = Post::where('serial', $serial)->first();
        return view('game.rank', [
            'serial' => $serial,
            'post' => $post
        ]);
    }

}
