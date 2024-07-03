<?php

namespace App\Extensions;

use Illuminate\Auth\Passwords\PasswordBroker as BasePasswordBroker;
use Closure;

class CustomPasswordBroker extends BasePasswordBroker
{
    /**
     * Send a password reset link to a user.
     *
     * @param  array  $credentials
     * @param  \Closure|null  $callback
     * @return string
     */
    public function sendResetLink(array $credentials, Closure $callback = null)
    {
        $response = parent::sendResetLink($credentials, $callback);

        if ($response === static::RESET_THROTTLED) {
            return $response;
        }

        return static::RESET_LINK_SENT;
    }
}
