<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\App;

class DefaultLocaleRoute
{
    /**
     * Keep unprefixed canonical routes, especially `/`, in the default locale.
     * A previous session preference must not change the language at a public URL.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @return mixed
     */
    public function handle(Request $request, Closure $next)
    {
        App::setLocale(config('app.default_public_locale'));

        return $next($request);
    }
}
