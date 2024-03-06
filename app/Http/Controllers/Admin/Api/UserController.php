<?php

namespace App\Http\Controllers\Admin\Api;

use App\Enums\Role;
use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use App\Models\User;

class UserController extends Controller
{

    public function ban(Request $request, $userId)
    {
        /**
         * @var User $user
         */
        $user = User::findOrFail($userId);
        $user->roles()->syncWithoutDetaching(find_role_id(Role::BANNED));
        CacheService::rememberUserRole($user, true);
        return response()->json();        
    }

    public function unban(Request $request, $userId)
    {
            /**
         * @var User $user
         */
        $user = User::findOrFail($userId);
        $user->roles()->detach(find_role_id(Role::BANNED));
        CacheService::rememberUserRole($user, true);
        return response()->json();        
    }

}
