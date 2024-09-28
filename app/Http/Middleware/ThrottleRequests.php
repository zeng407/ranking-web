<?php

namespace App\Http\Middleware;

use App\Helper\ClientRequestResolver;
use Closure;
use Illuminate\Http\Request;
use \Illuminate\Routing\Middleware\ThrottleRequests as BaseThrottleRequests;
use RuntimeException;

class ThrottleRequests extends BaseThrottleRequests
{
    use ClientRequestResolver;

    /**
     * Resolve request signature.
     *
     * @param  \Illuminate\Http\Request  $request
     * @return string
     *
     * @throws \RuntimeException
     */
    protected function resolveRequestSignature($request)
    {
        if ($user = $request->user()) {
            return sha1($user->getAuthIdentifier());
        } elseif ($route = $request->route()) {
            $ip = $this->getClientIp($request);
            return sha1($route->getDomain().'|'.$ip);
        }

        throw new RuntimeException('Unable to generate the request signature. Route unavailable.');
    }
}
