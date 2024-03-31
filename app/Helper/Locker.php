<?php

namespace App\Helper;

use App\Models\Game;
use App\Models\User;
use Cache;

class Locker
{
    static public function lockUpdateGameElement(Game $game)
    {
        logger('lock acquired', ['game_id' => $game->id]);
        return Cache::lock('lock_update_game_element_' . $game->id, 10);
    }
}