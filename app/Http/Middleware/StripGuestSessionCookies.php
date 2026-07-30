<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class StripGuestSessionCookies
{
    public const ENABLED_ATTRIBUTE = 'strip_guest_session_cookies';

    /**
     * Keep a read-only guest bootstrap request stateless. This middleware is
     * global so it runs after StartSession and VerifyCsrfToken add cookies.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @return mixed
     */
    public function handle(Request $request, Closure $next)
    {
        $hadSessionCookie = $request->cookies->has(config('session.cookie'));
        $response = $next($request);

        if ($request->attributes->get(self::ENABLED_ATTRIBUTE, false)
            && !$hadSessionCookie
            && !$request->user()) {
            $response->headers->remove('Set-Cookie');
        }

        return $response;
    }
}
