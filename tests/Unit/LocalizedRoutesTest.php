<?php

use Tests\TestCase;
use App\Http\Middleware\DefaultLocaleRoute;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\App;

class LocalizedRoutesTest extends TestCase
{
    public function test_localized_routes_use_the_new_public_prefixes(): void
    {
        $route = app('router')->getRoutes()->getByName('localized.home');

        $this->assertNotNull($route);
        $this->assertSame('{locale}', $route->uri());
        $this->assertSame('zh-tw|en|ja', $route->wheres['locale']);
        $this->assertContains('locale.prefix', $route->gatherMiddleware());
    }

    public function test_locale_prefix_maps_to_the_existing_translation_locale(): void
    {
        $this->get('/zh-tw/tos')
            ->assertOk()
            ->assertSee('服務條款');

        $this->get('/en/tos')
            ->assertOk()
            ->assertSee('Terms of Service');

        // The existing Japanese legal-page contract falls back to English.
        $this->get('/ja/tos')
            ->assertOk()
            ->assertSee('Terms of Service');
    }

    public function test_legacy_locale_urls_redirect_permanently_and_keep_the_query(): void
    {
        $this->get('/lang/zh_TW/donate?source=legacy')
            ->assertStatus(301)
            ->assertRedirect('/zh-tw/donate?source=legacy');

        $this->get('/lang/en')
            ->assertStatus(301)
            ->assertRedirect('/en/');
    }

    public function test_unprefixed_static_pages_redirect_to_traditional_chinese(): void
    {
        $this->get('/donate')->assertStatus(301)->assertRedirect('/zh-tw/donate');
        $this->get('/tos')->assertStatus(301)->assertRedirect('/zh-tw/tos');
        $this->get('/privacy')->assertStatus(301)->assertRedirect('/zh-tw/privacy');
    }

    public function test_locale_path_helpers_use_lowercase_hyphenated_prefixes(): void
    {
        $this->assertSame('/zh-tw/', localized_path('zh_TW'));
        $this->assertSame('/en/privacy', localized_path('en', '/privacy'));
        $this->assertSame('/ja/g/example', localized_path('ja', 'g/example'));
    }

    public function test_unprefixed_home_ignores_a_previous_session_locale(): void
    {
        App::setLocale('en');

        (new DefaultLocaleRoute())->handle(Request::create('/', 'GET'), function () {
            $this->assertSame('zh_TW', App::getLocale());
            return response('ok');
        });
    }
}
