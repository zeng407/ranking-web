<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;


/**
 * @mixin IdeHelperRank
 */
class Rank extends Model
{
    use HasFactory;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'post_id',
        'element_id',
        'rank_type',
        'record_date',
        'win_count',
        'round_count',
        'win_rate'
    ];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }

    public function element()
    {
        return $this->belongsTo(Element::class)->withTrashed();
    }


}
