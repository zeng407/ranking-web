<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Sanctum\HasApiTokens;
use Illuminate\Support\Facades\Cache;
use App\Helper\CacheService;

/**
 * @mixin IdeHelperUser
 */
class User extends Authenticatable
{
    use HasApiTokens, HasFactory, Notifiable;

    /**
     * The attributes that are mass assignable.
     *
     * @var string[]
     */
    protected $fillable = [
        'name',
        'name_updated_at',
        'email',
        'password',
        'avatar_url',
    ];

    /**
     * The attributes that should be hidden for serialization.
     *
     * @var array
     */
    protected $hidden = [
        'password',
        'remember_token',
    ];

    /**
     * The attributes that should be cast.
     *
     * @var array
     */
    protected $casts = [
        'email_verified_at' => 'datetime',
        'name_updated_at' => 'datetime',
    ];

    /**
     * Relationships
     */

    public function posts()
    {
        return $this->hasMany(Post::class);
    }

    public function roles()
    {
        return $this->belongsToMany(Role::class, UserRole::class)->withTimestamps();
    }

    public function user_socialite()
    {
        return $this->hasOne(UserSocialite::class);
    }

    /**
     * Functions
     */

    public function isAdmin()
    {
        return in_array(\App\Enums\Role::ADMIN, CacheService::rememberUserRole($this));
    }

    public function isBanned()
    {
        return in_array(\App\Enums\Role::BANNED, CacheService::rememberUserRole($this));
    }

    


}
