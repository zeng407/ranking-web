<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;

class GameController extends Controller
{

    public function show($serial)
    {
        return view('game.show', compact('serial'));
    }

    public function rank($serial)
    {
        return view('game.rank', compact('serial'));
    }

}
