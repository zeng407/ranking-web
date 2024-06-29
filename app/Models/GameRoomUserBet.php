<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperGameRoomUserBet
 */
class GameRoomUserBet extends Model
{
    use HasFactory;

    protected $fillable = [
        'game_room_id',
        'game_room_user_id',
        'current_round',
        'of_round',
        'remain_elements',
        'winner_id',
        'loser_id',
        'won_at',
        'lost_at',
        'last_combo',
        'score',
    ];
}
