<?php

use Illuminate\Session\Middleware\StartSession;
use Tests\TestCase;

class PublicHtmlRoutesTest extends TestCase
{
    public function test_home_and_rank_routes_use_public_html_cache_profiles(): void
    {
        $homeRoute = app('router')->getRoutes()->getByName('home');
        $rankRoute = app('router')->getRoutes()->getByName('game.rank');

        $this->assertNotNull($homeRoute);
        $this->assertNotNull($rankRoute);
        $this->assertContains('cache.public-html:home', $homeRoute->gatherMiddleware());
        $this->assertContains('cache.public-html:rank', $rankRoute->gatherMiddleware());
    }

    public function test_static_information_routes_use_public_html_cache(): void
    {
        $routes = collect(app('router')->getRoutes()->getRoutes());

        foreach ([
            'donate',
            'tos',
            'privacy',
            'lang/{locale}/donate',
            'lang/{locale}/tos',
            'lang/{locale}/privacy',
        ] as $uri) {
            $route = $routes->first(fn ($route) => $route->uri() === $uri);

            $this->assertNotNull($route, "Missing route: {$uri}");
            $this->assertContains('cache.public-html', $route->gatherMiddleware(), $uri);
        }
    }

    public function test_session_context_route_is_not_publicly_cacheable(): void
    {
        $route = app('router')->getRoutes()->getByName('session.context');

        $this->assertNotNull($route);
        $this->assertSame('session-context', $route->uri());
        $this->assertContains('web', $route->gatherMiddleware());
        $this->assertNotContains('api', $route->gatherMiddleware());
        $this->assertNotContains('cache.public-html', $route->gatherMiddleware());
        $this->assertNotContains('public-api', $route->gatherMiddleware());

        $resolvedMiddleware = app('router')->gatherRouteMiddleware($route);
        $this->assertContains(StartSession::class, $resolvedMiddleware);
    }
}
