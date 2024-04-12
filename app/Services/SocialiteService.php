<?php

namespace App\Services;

use App\Exceptions\UserSocialiteAlreadyConnected;
use App\Exceptions\UserSocialiteEmailExists;
use App\Models\UserSocialite;
use Laravel\Socialite\Facades\Socialite;
use App\Models\User;

class SocialiteService
{
    /**
     * Handle Google callback
     *
     * @return User
     * @throws UserSocialiteEmailExists
     */
    public function handleGoogleCallback() : User
    {
        $googleUser = Socialite::driver('google')->user();
        logger(json_encode($googleUser));
        $socialite = UserSocialite::where('google_id', $googleUser->id)->first();
        if ($socialite) {
            logger('socialite exists');
            return $socialite->user;
        }

        if (User::where('email', $googleUser->email)->exists()) {
            logger('email exists');
            throw new UserSocialiteEmailExists('Email already exists.');
        }

        try {
            \DB::beginTransaction();
            $user = User::create([
                'name' => $googleUser->nickname ?? $googleUser->name,
                'email' => $googleUser->email,
                'password' => '',
                'avatar_url' => $googleUser->avatar,
                'email_verified_at' => $googleUser->user['email_verified'] ? now() : null, 
            ]);
            $user->user_socialite()->create([
                'google_email' => $googleUser->email,
                'google_id' => $googleUser->id,
                'google_token' => $googleUser->token,
                'google_refresh_token' => $googleUser->refreshToken,
            ]);
            \DB::commit();
        } catch (\Exception $e) {
            \DB::rollBack();
            report($e);
            throw $e;
        }
        
        return $user;
    }

    public function handleGoogleConnect() : User
    {
        $googleUser = Socialite::driver('google')->user();
        if (UserSocialite::where('google_id', $googleUser->id)->exists()) {
            throw new UserSocialiteAlreadyConnected();
        }

        $user = \Auth::user();
        if ($user->user_socialite->google_id ?? false) {
            throw new UserSocialiteAlreadyConnected();
        }

        try {
            \DB::beginTransaction();
            $user->user_socialite()->create([
                'google_email' => $googleUser->email,
                'google_id' => $googleUser->id,
                'google_token' => $googleUser->token,
                'google_refresh_token' => $googleUser->refreshToken,
            ]);
            \DB::commit();
        } catch (\Exception $e) {
            \DB::rollBack();
            report($e);
            throw $e;
        }
        
        return $user;
    }
}