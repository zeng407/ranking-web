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
                'api_token' => null,
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
            'api_token' => null,
            'user' => [
                'avatar_url' => 'https://cdn.example.test/avatar.webp',
            ],
        ]);
    }

    public function test_authenticated_context_can_issue_an_ed25519_token_for_go(): void
    {
        if (!function_exists('sodium_crypto_sign_keypair')) {
            $this->markTestSkipped('The sodium extension is not installed.');
        }

        $keyPair = sodium_crypto_sign_keypair();
        $privateKey = sodium_crypto_sign_secretkey($keyPair);
        $publicKey = sodium_crypto_sign_publickey($keyPair);
        config()->set('services.go_auth.private_key', base64_encode($privateKey));
        config()->set('services.go_auth.issuer', 'https://2pick.app');
        config()->set('services.go_auth.audience', '2pick-go-api');
        config()->set('services.go_auth.ttl_seconds', 300);

        $user = User::factory()->create();
        $response = $this->actingAs($user)->getJson(route('session.context'));

        $response->assertOk()
            ->assertJsonPath('authenticated', true)
            ->assertJsonPath('api_token.token_type', 'Bearer')
            ->assertJsonPath('api_token.expires_in', 300);

        $token = $response->json('api_token.access_token');
        $parts = explode('.', $token);
        $this->assertCount(3, $parts);

        $header = json_decode($this->base64UrlDecode($parts[0]), true, 512, JSON_THROW_ON_ERROR);
        $claims = json_decode($this->base64UrlDecode($parts[1]), true, 512, JSON_THROW_ON_ERROR);
        $signature = $this->base64UrlDecode($parts[2]);

        $this->assertSame('EdDSA', $header['alg']);
        $this->assertSame('at+jwt', $header['typ']);
        $this->assertSame('https://2pick.app', $claims['iss']);
        $this->assertSame('2pick-go-api', $claims['aud']);
        $this->assertSame((string) $user->id, $claims['sub']);
        $this->assertSame(300, $claims['exp'] - $claims['iat']);
        $this->assertTrue(sodium_crypto_sign_verify_detached($signature, $parts[0].'.'.$parts[1], $publicKey));
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

    private function base64UrlDecode(string $value): string
    {
        $padding = (4 - strlen($value) % 4) % 4;
        return base64_decode(strtr($value.str_repeat('=', $padding), '-_', '+/'), true);
    }
}
