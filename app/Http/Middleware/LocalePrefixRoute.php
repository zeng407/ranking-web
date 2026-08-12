<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Session;
use App;

class LocalePrefixRoute
{
    /**
     * Handle an incoming request.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure(\Illuminate\Http\Request): (\Illuminate\Http\Response|\Illuminate\Http\RedirectResponse)  $next
     * @return \Illuminate\Http\Response|\Illuminate\Http\RedirectResponse
     */
    public function handle(Request $request, Closure $next)
    {
        if ($urlLocale = $request->route('locale')) {
            $locale = array_search(
                strtolower($urlLocale),
                config('app.locale_url_prefixes'),
                true
            );

            if ($locale === false) {
                return redirect()->route('home');
            }

            Session::put('locale', $locale);
            App::setLocale($locale);
        }
        return $next($request);
    }
}
