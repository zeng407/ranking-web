<?php

namespace App\Http\Controllers;

use App\Http\Middleware\StripGuestSessionCookies;
use Illuminate\Http\Request;
use Illuminate\Validation\Rule;

class SessionContextController extends Controller
{
    public function __invoke(Request $request)
    {
        $validated = $request->validate([
            'locale' => ['nullable', 'string', Rule::in(config('app.locales'))],
        ]);
        $requestedLocale = $validated['locale'] ?? null;

        if ($requestedLocale) {
            app()->setLocale($requestedLocale);
            $request->session()->put('locale', $requestedLocale);
        } else {
            $request->attributes->set(StripGuestSessionCookies::ENABLED_ATTRIBUTE, true);
        }

        $user = $request->user();

        return response()->json([
            'authenticated' => $user !== null,
            'csrf_token' => csrf_token(),
            'locale' => app()->getLocale(),
            'user' => $user ? [
                'avatar_url' => $user->avatar_url,
            ] : null,
        ])->withHeaders([
            'Cache-Control' => 'private, no-store',
            'Cloudflare-CDN-Cache-Control' => 'no-store',
        ]);
    }
}
