<?php

namespace App\Helper;

use App\Models\User;
use Cache;
use App\Enums\Role;


class CacheService
{
    static public function rememberUserRole(User $user, $refresh = false)
    {
        if ($refresh) {
            Cache::forget('user_role_' . $user->id);
        }
        return Cache::remember('user_role_' . $user->id, 60, function() use ($user) {
            return $user->roles()->pluck('slug')->toArray();
        });
    }
}