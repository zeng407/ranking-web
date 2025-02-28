<?php

namespace App\Models;

use App\Enums\PostAccessPolicy;
use App\Enums\TrendTimeRange;
use App\Models\Traits\HasImgurAlbum;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;


/**
 * @mixin IdeHelperPost
 */
class PublicPost extends Model
{
    use HasFactory;

    protected $fillable = [
        'post_id',
        'new_position',
        'day_position',
        'week_position',
        'month_position',
        'title',
        'description',
        'tags',
        'data'
    ];

    /**
     * The attributes that should be cast to native types.
     *
     * @var array
     */
    protected $casts = [
        'data' => 'array',
        'tags' => 'array'
    ];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }

}
