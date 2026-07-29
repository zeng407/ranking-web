<?php

use App\Http\Middleware\PublicApiCacheHeaders;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Tests\TestCase;

class PublicApiCacheHeadersTest extends TestCase
{
    public function test_it_sets_public_browser_and_cloudflare_cache_headers(): void
    {
        $request = Request::create('/api/posts', 'GET');
        $response = (new PublicApiCacheHeaders())->handle($request, function () {
            return new Response([], 200, ['Cache-Control' => 'no-cache, private']);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('public'));
        $this->assertSame('0', $response->headers->getCacheControlDirective('max-age'));
        $this->assertFalse($response->headers->hasCacheControlDirective('private'));
        $this->assertFalse($response->headers->hasCacheControlDirective('no-store'));
        $this->assertSame(
            'public, max-age=60, stale-while-revalidate=30, stale-if-error=3600',
            $response->headers->get('Cloudflare-CDN-Cache-Control')
        );
    }

    public function test_it_prevents_caching_when_controller_marks_response_private(): void
    {
        $request = Request::create('/api/rank', 'GET');
        $request->attributes->set(PublicApiCacheHeaders::CACHEABLE_ATTRIBUTE, false);

        $response = (new PublicApiCacheHeaders())->handle($request, function () {
            return new Response([], 200);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('private'));
        $this->assertTrue($response->headers->hasCacheControlDirective('no-store'));
        $this->assertFalse($response->headers->hasCacheControlDirective('public'));
        $this->assertSame('no-store', $response->headers->get('Cloudflare-CDN-Cache-Control'));
    }

    public function test_it_does_not_cache_error_responses(): void
    {
        $request = Request::create('/api/posts', 'GET');
        $response = (new PublicApiCacheHeaders())->handle($request, function () {
            return new Response([], 422);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('private'));
        $this->assertTrue($response->headers->hasCacheControlDirective('no-store'));
        $this->assertFalse($response->headers->hasCacheControlDirective('public'));
        $this->assertSame('no-store', $response->headers->get('Cloudflare-CDN-Cache-Control'));
    }
}
