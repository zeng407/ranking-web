<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\SoftDeletes;

/**
 * @mixin IdeHelperComment
 */
class Comment extends Model
{
    use HasFactory;
    use SoftDeletes;

    protected $fillable = [
        'user_id',
        'content',
        'anonymous_id',
        'nickname',
        'label',
        'anonymous_mode',
        'delete_hash',
        'ip',
        'edited_at'
    ];
    
    protected $casts = [
        'edited_at' => 'datetime',
        'label' => 'array'
    ];

    protected $with = ['user'];

    /**
     * Relationships
     */

    public function user()
    {
        return $this->belongsTo(User::class);
    }

    public function posts()
    {
        return $this->belongsToMany(Post::class, 'post_comments');
    }

    public function abuse_reports()
    {
        return $this->hasMany(ReportedComment::class);
    }

    /**
     * Functions
     */
    public function getPost()
    {
        return $this->posts()->first();
    }

    public function getChampions()
    {
        return $this->label['champions'] ?? [];
    }
}
