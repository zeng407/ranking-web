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
        if ($locale = $request->route('locale')) {
            // validate locale
            if (in_array($locale, ['en', 'zh_TW'])) {
                Session::put('locale', $locale);
                App::setLocale($locale);
            }                    
        }

        return $next($request);
    }
}
