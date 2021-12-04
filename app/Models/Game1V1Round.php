<?php

namespace App\Models;

use Illuminate\Contracts\Auth\MustVerifyEmail;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Sanctum\HasApiTokens;

/**
 * @mixin IdeHelperGame1V1Round
 */
class Game1V1Round extends Model
{
    use HasFactory;

    protected $table = 'game_1v1_rounds';

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'current_round',
        'of_round',
        'remain_elements',
        'winner_id',
        'loser_id',
        'complete_at',
    ];

    public function game()
    {
        return $this->belongsTo(Game::class);
    }

    public function winner()
    {
        return $this->belongsTo(Element::class, 'winner_id');
    }

    public function loser()
    {
        return $this->belongsTo(Element::class,'loser_id');
    }
}
