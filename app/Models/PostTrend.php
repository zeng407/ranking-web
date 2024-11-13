<?php

namespace App\Models;

use App\Enums\PostAccessPolicy;
use Illuminate\Contracts\Auth\MustVerifyEmail;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Sanctum\HasApiTokens;


/**
 * @mixin IdeHelperPostTrend
 */
class PostTrend extends Model
{
    use HasFactory;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'post_id',
        'trend_type',
        'time_range',
        'position',
        'start_date'
    ];


    /**
     * relations
     */
    public function post()
    {
        return $this->belongsTo(Post::class);
    }
}
