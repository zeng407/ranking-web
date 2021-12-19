<?php


namespace App\Http\Controllers\Post;


use App\Http\Controllers\Controller;
use Illuminate\Http\Request;

class GameController extends Controller
{

    public function show($serial)
    {
        return view('game.show', compact('serial'));
    }

    public function rank(Request $request, $serial)
    {

        return view('game.rank', compact('serial'));
    }

}
