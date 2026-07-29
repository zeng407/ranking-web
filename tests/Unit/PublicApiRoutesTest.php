<?php

use App\Enums\PostAccessPolicy;
use Tests\TestCase;
use Tests\TestHelper;

class PublicApiRoutesTest extends TestCase
{
    use TestHelper;

    public function test_public_api_does_not_start_a_session_for_same_origin_requests(): void
    {
        $response = $this
            ->withHeader('Referer', config('app.url').'/')
            ->getJson(route('api.tag.hot'));

        $response->assertOk();
        $response->assertCookieMissing('XSRF-TOKEN');
        $response->assertCookieMissing(config('session.cookie'));
        $this->assertTrue($response->headers->hasCacheControlDirective('public'));
    }

    public function test_only_selected_routes_use_the_public_api_middleware(): void
    {
        $publicRouteNames = [
            'api.rank.search',
            'api.rank.index',
            'api.public-post.index',
            'api.champion.index',
            'api.tag.index',
            'api.tag.hot',
            'api.carousel.index',
        ];

        foreach ($publicRouteNames as $routeName) {
            $route = app('router')->getRoutes()->getByName($routeName);

            $this->assertNotNull($route);
            $this->assertContains('public-api', $route->gatherMiddleware());
        }

        $privateRoute = app('router')->getRoutes()->getByName('api.private.rank.index');
        $this->assertNotNull($privateRoute);
        $this->assertNotContains('public-api', $privateRoute->gatherMiddleware());
        $this->assertContains('api', $privateRoute->gatherMiddleware());
    }

    public function test_private_rank_uses_the_stateful_endpoint_only(): void
    {
        $post = $this->createPost();
        $post->post_policy->update(['access_policy' => PostAccessPolicy::PRIVATE]);

        $params = [
            'post_serial' => $post->serial,
            'keyword' => 'no-match',
        ];

        $this->getJson(route('api.rank.search', $params))->assertNotFound();

        $this->be($post->user);
        $this->getJson(route('api.private.rank.search', $params))->assertOk();
    }
}
