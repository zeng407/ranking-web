<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Builder;

/**
 * @mixin IdeHelperRankReport
 */
class RankReport extends Model
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
        'rank',
        'final_win_position',
        'final_win_rate',
        'win_position',
        'win_rate',
        'hidden'
    ];

    protected $casts = [
        'hidden' => 'boolean',
    ];

    protected $with = ['element'];

    public function post()
    {
        return $this->belongsTo(Post::class)->withTrashed();
    }

    public function element()
    {
        return $this->belongsTo(Element::class)->withTrashed();
    }

    public function rank_report_histories()
    {
        return $this->hasMany(RankReportHistory::class);
    }

    public function scopeVisible($query)
    {
        return $query->where('hidden', false);
    }

    protected static function booted()
    {
        // 定義一個名為 'visible' 的全域 Scope
        static::addGlobalScope('visible', function (Builder $builder) {
            $builder->where('hidden', false);
        });
    }

}
