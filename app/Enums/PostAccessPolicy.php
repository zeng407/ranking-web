<?php


namespace App\Enums;

enum PostAccessPolicy: string
{
    const PRIVATE = 'private';
    const PUBLIC = 'public';
    const PASSWORD = 'password';

    public static function trans($value)
    {
        $key = 'enum.post_access_policy.' . $value;
        if(!trans()->has($key, null, false)){
            \Log::warning("miss translation $key");
            return $value;
        }
        return __($key);
    }
}

