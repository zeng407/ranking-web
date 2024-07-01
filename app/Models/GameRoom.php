<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperGameRoom
 */
class GameRoom extends Model
{
    use HasFactory;

    protected $fillable = ['game_id', 'serial'];

    public function game()
    {
        return $this->belongsTo(Game::class);
    }

    public function users()
    {
        return $this->hasMany(GameRoomUser::class);
    }

    public function bets()
    {
        return $this->hasMany(GameRoomUserBet::class);
    }
}
