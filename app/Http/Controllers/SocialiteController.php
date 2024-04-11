<?php

namespace App\Http\Controllers;

use App\Services\SocialiteService;
use Illuminate\Http\Request;
use Laravel\Socialite\Facades\Socialite;

class SocialiteController extends Controller
{

    protected $socialiteService;

    public function __construct(SocialiteService $socialiteService)
    {
        $this->socialiteService = $socialiteService;
    }

    public function redirectGoogle()
    {
        return Socialite::driver('google')->redirect();
    }

    public function connectGoogle()
    {
        if (!\Auth::check()) {
            return redirect()->route('login');
        }
        session()->put(\Auth::id() . 'connectGoogle', true);

        return Socialite::driver('google')->redirect();
    }

    public function callbackGoogle(Request $request)
    {
        try {
            return $this->handleGoogleConnectOrCallback();
        } catch (\App\Exceptions\UserSocialiteEmailExists $e) {
            return $this->handleException($e, $request);
        } catch (\App\Exceptions\UserSocialiteAlreadyConnected $e) {
            return $this->handleException($e, $request);
        } catch (\Exception $e) {
            return $this->handleGeneralException($e);
        }
    }

    private function handleGoogleConnectOrCallback()
    {
        if (session()->pull(\Auth::id() . 'connectGoogle')) {
            $user = $this->socialiteService->handleGoogleConnect();
            return redirect()->route('profile.index')->with('success', __('socialite.connect.success'));
        } else {
            $user = $this->socialiteService->handleGoogleCallback();
            \Auth::login($user);
            return redirect()->route('home');
        }
    }

    private function handleException($exception, $request)
    {
        return $exception->render($request);
    }

    private function handleGeneralException($exception)
    {
        report($exception);
        return redirect()->route('login')->with('error', __('socialite.failed'));
    }
}
