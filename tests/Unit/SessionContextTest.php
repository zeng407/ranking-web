<?php

use App\Models\User;
use Tests\TestCase;

class SessionContextTest extends TestCase
{
    public function test_guest_context_is_private_and_contains_a_csrf_token(): void
    {
        $response = $this->getJson(route('session.context'));

        $response->assertOk()
            ->assertJson([
                'authenticated' => false,
                'locale' => config('app.locale'),
                'user' => null,
            ]);

        $this->assertNotEmpty($response->json('csrf_token'));
        $this->assertTrue($response->headers->hasCacheControlDirective('private'));
        $this->assertTrue($response->headers->hasCacheControlDirective('no-store'));
        $this->assertSame('no-store', $response->headers->get('Cloudflare-CDN-Cache-Control'));
        $this->assertFalse($response->headers->has('Set-Cookie'));
    }

    public function test_authenticated_context_returns_only_navbar_user_data(): void
    {
        $user = User::factory()->create([
            'avatar_url' => 'https://cdn.example.test/avatar.webp',
        ]);

        $response = $this->actingAs($user)->getJson(route('session.context'));

        $response->assertOk()->assertExactJson([
            'authenticated' => true,
            'csrf_token' => $response->json('csrf_token'),
            'locale' => config('app.locale'),
            'user' => [
                'avatar_url' => 'https://cdn.example.test/avatar.webp',
            ],
        ]);
    }

    public function test_non_default_locale_context_persists_its_session_locale(): void
    {
        $response = $this->getJson(route('session.context', ['locale' => 'en']));

        $response->assertOk()->assertJson([
            'authenticated' => false,
            'locale' => 'en',
        ]);

        $this->assertSame('en', session('locale'));
    }
}
