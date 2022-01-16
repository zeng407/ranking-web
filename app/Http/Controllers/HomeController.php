<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;

class HomeController extends Controller
{
    public function index()
    {
        return view('home', [
            'sort' => 'hot'
        ]);
    }

    public function hot()
    {
        return view('home', [
            'sort' => 'hot'
        ]);
    }

    public function new()
    {
        return view('home', [
            'sort' => 'new'
        ]);
    }
}
