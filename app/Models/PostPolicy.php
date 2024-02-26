<?php

namespace App\Models;

use App\Enums\PostAccessPolicy;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;


/**
 * @mixin IdeHelperPostPolicy
 */
class PostPolicy extends Model
{
    use HasFactory;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'access_policy',
        'password'
    ];

    public function post()
    {
        return $this->belongsTo(Post::class);
    }

    public function getAccessPolicyEnum()
    {
        return PostAccessPolicy::trans($this->access_policy);
    }
}
