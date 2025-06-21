<?php

namespace App\Http\Controllers\Api;

use App\Helper\CacheService;
use App\Http\Controllers\Controller;

class AdController extends Controller
{
    public function removeAd24hr()
    {
        CacheService::setSkipAds();
        return response()->json(['success' => true]);
    }
}
