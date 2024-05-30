<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;

/**
 * @mixin IdeHelperRankReportHistory
 */
class RankReportHistory extends Model
{
    use HasFactory;
    use SoftDeletes;

    protected $fillable = [
        'post_id',
        'element_id',
        'rank_report_id',
        'time_range',
        'start_date',
        'rank',
        'win_rate',
        'win_count',
        'lose_count',
        'champion_count',
        'game_complete_count',
        'champion_rate'
    ];

    public function rank_report()
    {
        return $this->belongsTo(RankReport::class);
    }

    public function element()
    {
        return $this->belongsTo(Element::class);
    }

    public function post()
    {
        return $this->belongsTo(Post::class);
    }
}
