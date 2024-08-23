<?php

namespace App\Helper;

use App\Models\Game;
use App\Models\User;
use Cache;
use Illuminate\Http\Request;

trait ClientRequestResolver
{
    public function getClientIp(Request $request)
    {
        return $request->header('CF-Connecting-IP') ?? $request->ip();

    }

    public function getClientIpContry(Request $request)
    {
        return $request->header('CF-IPCountry') ?? null;
    }

}
