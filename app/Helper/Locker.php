<?php

namespace App\Helper;

use App\Models\Game;
use App\Models\User;
use Cache;

class Locker
{
    static public function lockUpdateGameElement(Game $game)
    {
        // logger('lock acquired', ['game_id' => $game->id]);
        return Cache::lock('lock_update_game_element_' . $game->id, 10);
    }

    static public function lockRankJob($postId)
    {
        // logger('lock acquired', ['post_id' => $postId]);
        return Cache::lock('lock_rank_job_' . $postId, 10);
    }

    static public function lockRankHistory($postId)
    {
        return Cache::lock('lock_rank_history_' . $postId, 10);
    }
}
