<?php

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
        $this->assertContains('locale.default', $homeRoute->gatherMiddleware());
    }

    public function test_static_information_routes_use_public_html_cache(): void
    {
        $routes = collect(app('router')->getRoutes()->getRoutes());

        foreach ([
            '{locale}/donate',
            '{locale}/tos',
            '{locale}/privacy',
        ] as $uri) {
            $route = $routes->first(fn ($route) => $route->uri() === $uri);

            $this->assertNotNull($route, "Missing route: {$uri}");
            $this->assertContains('cache.public-html', $route->gatherMiddleware(), $uri);
        }
    }
}
