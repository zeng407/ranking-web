<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;

/**
 * @mixin IdeHelperHomeCarousel
 */
class HomeCarouselItem extends Model
{
    use HasFactory;
    use SoftDeletes;

    protected $fillable = [
        'title',
        'description',
        'image_url',
        'video_url',
        'position',
        'type',
        'video_source',
        'video_id',
        'video_start_second',
        'video_end_second'
    ];
}
