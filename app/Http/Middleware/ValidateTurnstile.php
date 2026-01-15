<?php

namespace App\Http\Middleware;

use App\Helper\ClientRequestResolver;
use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;
use Symfony\Component\HttpFoundation\Response;

class ValidateTurnstile
{
    use ClientRequestResolver;

    public function handle(Request $request, Closure $next): Response
    {
        // 1. 如果使用者已經驗證過 (Session 存在)，直接放行
        if ($request->session()->has('is_human')) {
            return $next($request);
        }

        if ($request->isMethod('post') && $request->has('cf-turnstile-response')) {

            $response = Http::asForm()->post('https://challenges.cloudflare.com/turnstile/v0/siteverify', [
                'secret'   => config('services.cloudflare.turnstile_secret_key'),
                'response' => $request->input('cf-turnstile-response'),
                'remoteip' => $this->getClientIp($request),
            ]);

            if ($response->json('success')) {
                $request->session()->put('is_human', true);

                return redirect()->to($request->fullUrl());
            }

            return response('Turnstile verification failed. Please try again.', 403);
        }

        return $next($request);
    }
}
