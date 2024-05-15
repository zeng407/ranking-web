<?php

namespace App\Models;

use App\Enums\PostAccessPolicy;
use App\Models\Traits\HasImgurAlbum;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;


/**
 * @mixin IdeHelperPost
 */
class Post extends Model
{
    use HasImgurAlbum;
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

    protected $with = ['post_policy', 'tags', 'elements'];

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

    public function tags()
    {
        return $this->belongsToMany(Tag::class, 'post_tags')->withTimestamps();
    }

    public function comments()
    {
        return $this->belongsToMany(Comment::class, 'post_comments');
    }

    /**
     * functions
     */

    public function isPublic()
    {
        return $this->post_policy->access_policy === PostAccessPolicy::PUBLIC;
    }
    public function isPrivate()
    {
        return $this->post_policy->access_policy === PostAccessPolicy::PRIVATE;
    }
    public function isPasswordRequired()
    {
        return $this->post_policy->access_policy === PostAccessPolicy::PASSWORD;
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
