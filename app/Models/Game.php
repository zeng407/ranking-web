<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
/**
 * @mixin IdeHelperGame
 */
class Game extends Model
{
    use HasFactory;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'serial',
        'element_count'
    ];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }

    public function game_1v1_rounds()
    {
        return $this->hasMany(Game1V1Round::class);
    }

    public function elements()
    {
        return $this->belongsToMany(
            Element::class,
            'game_elements',
            'game_id',
            'element_id',
        )->withTrashed();
    }


}
