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
 * @mixin IdeHelperRankReport
 */
class RankReport extends Model
{
    use HasFactory;
    use SoftDeletes;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'post_id',
        'element_id',
        'rank',
        'final_win_position',
        'final_win_rate',
        'win_position',
        'win_rate'
    ];

    protected $with = ['element'];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }

    public function element()
    {
        return $this->belongsTo(Element::class)->withTrashed();
    }


}
