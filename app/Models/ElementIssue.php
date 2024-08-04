<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperElementIssue
 */
class ElementIssue extends Model
{
    use HasFactory;

    protected $fillable = [
        'element_id',
        'type',
        'resolved_at',
    ];

    public function element()
    {
        return $this->belongsTo(Element::class);
    }
}
