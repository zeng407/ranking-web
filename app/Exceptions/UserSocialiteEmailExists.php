<?php

namespace App\Exceptions;

use Exception;

class UserSocialiteEmailExists extends Exception
{
    public function render($request)
    {
        if(!\Auth::check()){
            return redirect()->route('login')->with('warning', __('Email already exists.'));
        }else{
            return redirect()->route('profile.index')->with('warning', __('Email already exists.'));
        }
    }
}
