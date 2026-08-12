<?php

namespace App\Services\Auth;

use App\Models\User;
use RuntimeException;

class GoAccessTokenService
{
    /**
     * Issue a short-lived Ed25519-signed token for the Go API.
     *
     * Returning null is intentional while the bridge is not configured: the
     * existing Laravel session remains fully functional during migration.
     */
    public function issue(User $user): ?array
    {
        $encodedPrivateKey = trim((string) config('services.go_auth.private_key', ''));
        if ($encodedPrivateKey === '') {
            return null;
        }

        if (!function_exists('sodium_crypto_sign_detached')) {
            throw new RuntimeException('The sodium PHP extension is required when GO_AUTH_PRIVATE_KEY is configured.');
        }

        $privateKey = base64_decode($encodedPrivateKey, true);
        if ($privateKey === false) {
            throw new RuntimeException('GO_AUTH_PRIVATE_KEY must be valid base64.');
        }

        if (strlen($privateKey) === SODIUM_CRYPTO_SIGN_SEEDBYTES) {
            $keyPair = sodium_crypto_sign_seed_keypair($privateKey);
            $privateKey = sodium_crypto_sign_secretkey($keyPair);
        }

        if (strlen($privateKey) !== SODIUM_CRYPTO_SIGN_SECRETKEYBYTES) {
            throw new RuntimeException('GO_AUTH_PRIVATE_KEY must contain a 32-byte Ed25519 seed or 64-byte secret key.');
        }

        $ttl = (int) config('services.go_auth.ttl_seconds', 300);
        if ($ttl < 60 || $ttl > 900) {
            throw new RuntimeException('GO_AUTH_TOKEN_TTL must be between 60 and 900 seconds.');
        }

        $now = now()->getTimestamp();
        $expiresAt = $now + $ttl;
        $header = [
            'alg' => 'EdDSA',
            'typ' => 'at+jwt',
            'kid' => (string) config('services.go_auth.key_id', 'primary'),
        ];
        $claims = [
            'iss' => (string) config('services.go_auth.issuer', config('app.url')),
            'aud' => (string) config('services.go_auth.audience', '2pick-go-api'),
            'sub' => (string) $user->getKey(),
            'roles' => $user->roles()->pluck('slug')->filter()->values()->all(),
            'iat' => $now,
            'nbf' => $now,
            'exp' => $expiresAt,
            'jti' => bin2hex(random_bytes(16)),
        ];

        $encodedHeader = $this->base64UrlEncode(json_encode($header, JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR));
        $encodedClaims = $this->base64UrlEncode(json_encode($claims, JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR));
        $signingInput = $encodedHeader.'.'.$encodedClaims;
        $signature = sodium_crypto_sign_detached($signingInput, $privateKey);

        return [
            'access_token' => $signingInput.'.'.$this->base64UrlEncode($signature),
            'token_type' => 'Bearer',
            'expires_in' => $ttl,
            'expires_at' => $expiresAt,
        ];
    }

    private function base64UrlEncode(string $value): string
    {
        return rtrim(strtr(base64_encode($value), '+/', '-_'), '=');
    }
}
