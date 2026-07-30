<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class PublicHtmlCacheHeaders
{
    public const ENABLED_ATTRIBUTE = 'public_html_cache_enabled';
    public const CACHEABLE_ATTRIBUTE = 'public_html_cacheable';

    public const CLOUDFLARE_CACHE_CONTROL =
        'public, max-age=60, stale-while-revalidate=30, stale-if-error=3600';

    /**
     * Finalize shared HTML cache headers after the web middleware has added
     * session and CSRF cookies to the response.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  \Closure  $next
     * @return mixed
     */
    public function handle(Request $request, Closure $next)
    {
        $hadSessionCookie = $request->cookies->has(config('session.cookie'));
        $response = $next($request);

        if (!$request->attributes->get(self::ENABLED_ATTRIBUTE, false)) {
            return $response;
        }

        $contentType = (string) $response->headers->get('Content-Type', '');
        $cacheable = in_array($request->method(), ['GET', 'HEAD'], true)
            && $response->getStatusCode() === 200
            && str_starts_with(strtolower($contentType), 'text/html')
            && !$hadSessionCookie
            && !$request->user()
            && $request->attributes->get(self::CACHEABLE_ATTRIBUTE, true) === true;

        if (!$cacheable) {
            $response->headers->set('Cache-Control', 'private, no-store', true);
            $response->headers->set('Cloudflare-CDN-Cache-Control', 'no-store', true);

            return $response;
        }

        // Browsers revalidate; Cloudflare may serve the shared copy briefly.
        $response->headers->set('Cache-Control', 'public, max-age=0, must-revalidate', true);
        $response->headers->set(
            'Cloudflare-CDN-Cache-Control',
            self::CLOUDFLARE_CACHE_CONTROL,
            true
        );
        $response->headers->remove('Set-Cookie');
        $response->headers->remove('Pragma');
        $response->headers->remove('Expires');

        return $response;
    }
}
