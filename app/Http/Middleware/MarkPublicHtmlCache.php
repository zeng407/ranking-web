<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class MarkPublicHtmlCache
{
    /**
     * Mark a route as eligible for shared HTML caching.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @param  string  $profile
     * @return mixed
     */
    public function handle(Request $request, Closure $next, string $profile = 'default')
    {
        $request->attributes->set(PublicHtmlCacheHeaders::ENABLED_ATTRIBUTE, true);
        $request->attributes->set(
            PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE,
            $this->hasOnlyCacheableQueryParameters($request, $profile)
        );

        return $next($request);
    }

    protected function hasOnlyCacheableQueryParameters(Request $request, string $profile): bool
    {
        $allowedParameters = [
            'home' => ['sort_by', 'range', 'page'],
            'rank' => ['page', 'tab'],
            'default' => [],
        ][$profile] ?? [];

        return count(array_diff(array_keys($request->query()), $allowedParameters)) === 0;
    }
}
