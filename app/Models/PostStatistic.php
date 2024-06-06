<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperPostStatistic
 */
class PostStatistic extends Model
{
    use HasFactory;

    protected $fillable = [
        'start_date',
        'play_count'
    ];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }
}
