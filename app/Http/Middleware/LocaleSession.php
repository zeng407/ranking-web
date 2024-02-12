<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Session;
use App;

class LocaleSession
{
    /**
     * Handle an incoming request.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @return mixed
     */
    public function handle($request, Closure $next)
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
