<?php

namespace App\Models;

use App\Models\Traits\HasImgurImage;
use Illuminate\Contracts\Auth\MustVerifyEmail;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Sanctum\HasApiTokens;

/**
 * @mixin IdeHelperElement
 */
class Element extends Model
{
    use HasFactory;
    use SoftDeletes;
    use HasImgurImage;


    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'path',
        'source_url',
        'thumb_url',
        'mediumthumb_url',
        'lowthumb_url',
        'title',
        'type',
        'video_source',
        'video_id',
        'video_duration_second',
        'video_start_second',
        'video_end_second'
    ];

    protected $with = ['imgur_image'];

    public function posts()
    {
        return $this->belongsToMany(
            Post::class,
            'post_elements',
            'element_id',
            'post_id'
        );
    }

    public function rank_reports()
    {
        return $this->hasMany(RankReport::class);
    }

    public function element_issues()
    {
        return $this->hasMany(ElementIssue::class);
    }

    public function getLowThumbUrl()
    {
        return $this->lowthumb_url ?: $this->thumb_url;
    }

    public function getMediumThumbUrl()
    {
        return $this->mediumthumb_url ?: $this->thumb_url;
    }

    public function getDefaultThumbUrl()
    {
        return $this->thumb_url;
    }

    public function getImgurUrl()
    {
        return $this->imgur_image?->link;
    }

}
