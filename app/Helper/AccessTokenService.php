<?php

namespace App\Helper;

use App\Models\Post;

class AccessTokenService
{
    const TOKEN_EXPIRED_MINUTES = 30;
    
    static public function setPostAccessToken(Post $post, $hashPassword)
    {
        logger('setPostAccessToken', [$post->serial, $hashPassword]);
        \Session::put($post->serial.'_post_access_token', [
            'value' => $hashPassword,
            'expired_at' => now()->addMinutes(self::TOKEN_EXPIRED_MINUTES)
        ]);
    }

    static public function getPostAccessToken(Post $post)
    {
        logger('getPostAccessToken', [$post->serial]);
        $accessToken = \Session::get($post->serial.'_post_access_token');
        logger('getPostAccessToken', [$accessToken]);
        if(!$accessToken){
            return null;
        }

        if(isset($accessToken['expired_at']) && now()->gt($accessToken['expired_at'])){
            \Session::forget($post->serial.'_post_access_token');
            return null;
        }

        return $accessToken['value'] ?? null;
    }

    static public function verifyPostAccessToken(Post $post)
    {
        $accessToken = self::getPostAccessToken($post);

        if(!$accessToken){
            return false;
        }

        logger('verifyPostAccessToken', [$post->serial, $accessToken]);
        $valid = $post->post_policy->password === $accessToken;
        logger('verifyPostAccessToken', [$valid, $post->post_policy->password, $accessToken]);
        return $valid;
    }

    static public function extendPostAccessToken(Post $post)
    {
        logger('extendPostAccessToken', [$post->serial]);
        $accessToken = self::getPostAccessToken($post);

        if(!$accessToken){
            return false;
        }

        self::setPostAccessToken($post, $accessToken);
        return true;
    }
}