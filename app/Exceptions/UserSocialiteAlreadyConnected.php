<?php

namespace App\Exceptions;

use Exception;

class UserSocialiteAlreadyConnected extends Exception
{
    public function render($request)
    {
        return redirect()->route('profile.index')->with('warning', __('Social media account is already connected.'));
    }
}
