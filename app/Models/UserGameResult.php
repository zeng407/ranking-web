<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperUserGameResult
 */
class UserGameResult extends Model
{
    use HasFactory;

    protected $fillable = [
        'user_id',
        'game_id',
        'champion_id',
        'champion_name',
        'anonymous_id'
    ];

    public function user()
    {
        return $this->belongsTo(User::class);
    }

    public function game()
    {
        return $this->belongsTo(Game::class);
    }

    public function champion()
    {
        return $this->belongsTo(Element::class, 'champion_id');
    }
}
