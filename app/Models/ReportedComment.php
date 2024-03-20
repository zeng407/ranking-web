<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperReportedComment
 */
class ReportedComment extends Model
{
    use HasFactory;

    protected $fillable = [
        'reporter_id',
        'reporter_ip',
        'comment_content',
        'reason'
    ];
}
