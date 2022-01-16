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
 * @mixin IdeHelperPost
 */
class Post extends Model
{
    use HasFactory;
    use SoftDeletes;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'title',
        'serial',
        'description',
    ];


    /**
     * relations
     */
    public function user()
    {
        return $this->belongsTo(User::class);
    }

    public function elements()
    {
        return $this->belongsToMany(Element::class, 'post_elements', 'post_id', 'element_id');
    }

    public function post_policy()
    {
        return $this->hasOne(PostPolicy::class);
    }

    public function games()
    {
        return $this->hasMany(Game::class);
    }

    public function ranks()
    {
        return $this->hasMany(Rank::class);
    }

    public function rank_reports()
    {
        return $this->hasMany(RankReport::class);
    }

    public function post_trends()
    {
        return $this->hasMany(PostTrend::class);
    }

    /**
     * functions
     */

    public function isPublic()
    {
        return $this->post_policy->access_policy === PostAccessPolicy::PUBLIC;
    }

    /**
     * scope
     */
    public function scopePublic($query)
    {
        return $query->whereHas('post_policy', function ($query) {
            $query->where('access_policy', PostAccessPolicy::PUBLIC);
        });
    }

}
