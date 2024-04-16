<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;

/**
 * @mixin IdeHelperUserSocialite
 */
class UserSocialite extends Model
{
    protected $table = 'user_socialities';

    use HasFactory;

    protected $fillable = [
        'google_email',
        'google_id',
        'google_token',
        'google_refresh_token',
    ];

    public function user()
    {
        return $this->belongsTo(User::class);
    }
}
