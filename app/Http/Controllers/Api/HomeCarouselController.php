<?php

namespace App\Http\Controllers\Api;

use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use App\Services\HomeCarouselService;
use Illuminate\Http\Request;
use App\Http\Resources\HomeCarouselItem\CarouselItemResource; 


class HomeCarouselController extends Controller
{
    protected HomeCarouselService $homeCarouselService;

    public function __construct(HomeCarouselService $homeCarouselService)
    {
        $this->homeCarouselService = $homeCarouselService;
    }

    public function index(Request $request)
    {
        $carouselItems = CacheService::rememberCarousels();
        return CarouselItemResource::collection($carouselItems);
    }
}
