<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class PublicApiCacheHeaders
{
    /**
     * Controllers may set this request attribute to false when a normally
     * public endpoint returns data whose authorization depends on a session.
     */
    public const CACHEABLE_ATTRIBUTE = 'public_api_cacheable';

    /**
     * Add browser and Cloudflare cache directives to a public API response.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @return mixed
     */
    public function handle(Request $request, Closure $next)
    {
        $response = $next($request);

        $cacheable = in_array($request->method(), ['GET', 'HEAD'], true)
            && $response->getStatusCode() === 200
            && $request->attributes->get(self::CACHEABLE_ATTRIBUTE, true) === true;

        if (!$cacheable) {
            $response->headers->set('Cache-Control', 'private, no-store', true);
            $response->headers->set('Cloudflare-CDN-Cache-Control', 'no-store', true);

            return $response;
        }

        // Browsers revalidate on each visit; Cloudflare keeps the shared copy.
        $response->headers->set('Cache-Control', 'public, max-age=0', true);
        $response->headers->set(
            'Cloudflare-CDN-Cache-Control',
            'public, max-age=60, stale-while-revalidate=30, stale-if-error=3600',
            true
        );
        $response->headers->remove('Pragma');
        $response->headers->remove('Expires');

        return $response;
    }
}
