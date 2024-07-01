<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperGameRoomUser
 */
class GameRoomUser extends Model
{
    use HasFactory;

    protected $fillable = [
        'game_room_id',
        'user_id',
        'anonymous_id',
        'nickname',
        'score',
        'rank',
        'accuracy',
        'total_played',
        'total_correct',
        'combo'
    ];

    public function gameRoom()
    {
        return $this->belongsTo(GameRoom::class);
    }

    public function user()
    {
        return $this->belongsTo(User::class);
    }

    public function bets()
    {
        return $this->hasMany(GameRoomUserBet::class);
    }
}
