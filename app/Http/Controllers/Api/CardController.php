<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;

class CardController extends Controller
{
    public function index()
    {
        $data = [
            ['title' => 'Card 1', 'content' => 'Card Content 1' ],
            ['title' => 'Card 2', 'content' => 'Card Content 2' ],
            ['title' => 'Card 3', 'content' => 'Card Content 3' ],
            ['title' => 'Card 4', 'content' => 'Card Content 4' ],
            ['title' => 'Card 5', 'content' => 'Card Content 5' ],
            ['title' => 'Card 6', 'content' => 'Card Content 6' ]
        ];
        
        return response()->json($data);
    }
}
