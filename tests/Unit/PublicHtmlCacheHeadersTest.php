<?php

use App\Http\Middleware\MarkPublicHtmlCache;
use App\Http\Middleware\PublicHtmlCacheHeaders;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\Route;
use Tests\TestCase;

class PublicHtmlCacheHeadersTest extends TestCase
{
    public function test_it_sets_short_cloudflare_cache_headers_and_removes_cookies(): void
    {
        $request = Request::create('/', 'GET');
        $request->attributes->set(PublicHtmlCacheHeaders::ENABLED_ATTRIBUTE, true);
        $request->attributes->set(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE, true);

        $response = (new PublicHtmlCacheHeaders())->handle($request, function () {
            return new Response('<html></html>', 200, [
                'Content-Type' => 'text/html; charset=UTF-8',
                'Set-Cookie' => 'ranking_web_session=test; path=/; httponly',
            ]);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('public'));
        $this->assertTrue($response->headers->hasCacheControlDirective('must-revalidate'));
        $this->assertSame('0', $response->headers->getCacheControlDirective('max-age'));
        $this->assertSame(
            'public, max-age=60, stale-while-revalidate=30, stale-if-error=3600',
            $response->headers->get('Cloudflare-CDN-Cache-Control')
        );
        $this->assertFalse($response->headers->has('Set-Cookie'));
    }

    public function test_global_finalizer_removes_cookies_added_by_web_middleware(): void
    {
        Route::middleware(['web', 'cache.public-html'])->get('/_test/cacheable-html', function () {
            session()->put('generated-during-request', true);

            return response('<html></html>', 200, ['Content-Type' => 'text/html']);
        });

        $response = $this->get('/_test/cacheable-html');

        $response->assertOk();
        $this->assertTrue($response->headers->hasCacheControlDirective('public'));
        $this->assertFalse($response->headers->has('Set-Cookie'));
        $this->assertSame(
            PublicHtmlCacheHeaders::CLOUDFLARE_CACHE_CONTROL,
            $response->headers->get('Cloudflare-CDN-Cache-Control')
        );
    }

    public function test_it_does_not_cache_requests_with_an_existing_session_cookie(): void
    {
        $request = Request::create('/', 'GET');
        $request->cookies->set(config('session.cookie'), 'existing-session');
        $request->attributes->set(PublicHtmlCacheHeaders::ENABLED_ATTRIBUTE, true);
        $request->attributes->set(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE, true);

        $response = (new PublicHtmlCacheHeaders())->handle($request, function () {
            return new Response('<html></html>', 200, ['Content-Type' => 'text/html']);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('private'));
        $this->assertTrue($response->headers->hasCacheControlDirective('no-store'));
        $this->assertSame('no-store', $response->headers->get('Cloudflare-CDN-Cache-Control'));
    }

    public function test_it_respects_a_controller_cache_bypass(): void
    {
        $request = Request::create('/r/private-post', 'GET');
        $request->attributes->set(PublicHtmlCacheHeaders::ENABLED_ATTRIBUTE, true);
        $request->attributes->set(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE, false);

        $response = (new PublicHtmlCacheHeaders())->handle($request, function () {
            return new Response('<html></html>', 200, ['Content-Type' => 'text/html']);
        });

        $this->assertTrue($response->headers->hasCacheControlDirective('private'));
        $this->assertTrue($response->headers->hasCacheControlDirective('no-store'));
        $this->assertSame('no-store', $response->headers->get('Cloudflare-CDN-Cache-Control'));
    }

    public function test_home_search_and_rank_result_queries_are_not_cacheable(): void
    {
        $homeRequest = Request::create('/?k=keyword', 'GET');
        (new MarkPublicHtmlCache())->handle($homeRequest, fn () => new Response(), 'home');

        $rankRequest = Request::create('/r/post?g=game-result', 'GET');
        (new MarkPublicHtmlCache())->handle($rankRequest, fn () => new Response(), 'rank');

        $this->assertFalse(
            $homeRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
        $this->assertFalse(
            $rankRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
    }

    public function test_bounded_home_and_rank_queries_remain_cacheable(): void
    {
        $homeRequest = Request::create('/?sort_by=hot&range=week&page=2', 'GET');
        (new MarkPublicHtmlCache())->handle($homeRequest, fn () => new Response(), 'home');

        $rankRequest = Request::create('/r/post?page=2&tab=1', 'GET');
        (new MarkPublicHtmlCache())->handle($rankRequest, fn () => new Response(), 'rank');

        $this->assertTrue(
            $homeRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
        $this->assertTrue(
            $rankRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
    }

    public function test_default_profile_only_caches_urls_without_query_parameters(): void
    {
        $plainRequest = Request::create('/donate', 'GET');
        (new MarkPublicHtmlCache())->handle($plainRequest, fn () => new Response());

        $queryRequest = Request::create('/donate?source=campaign', 'GET');
        (new MarkPublicHtmlCache())->handle($queryRequest, fn () => new Response());

        $this->assertTrue(
            $plainRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
        $this->assertFalse(
            $queryRequest->attributes->get(PublicHtmlCacheHeaders::CACHEABLE_ATTRIBUTE)
        );
    }
}
