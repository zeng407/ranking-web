<?php

namespace App\Observers;

use App\Helper\CacheService;
use App\Models\HomeCarouselItem;

class HomeCarouselItemObserver
{
    /**
     * Handle the HomeCarouselItem "created" event.
     *
     * @param  \App\Models\HomeCarouselItem  $homeCarouselItem
     * @return void
     */
    public function created(HomeCarouselItem $homeCarouselItem)
    {
        CacheService::clearCarousels();
    }

    /**
     * Handle the HomeCarouselItem "updated" event.
     *
     * @param  \App\Models\HomeCarouselItem  $homeCarouselItem
     * @return void
     */
    public function updated(HomeCarouselItem $homeCarouselItem)
    {
        CacheService::clearCarousels();
    }

    /**
     * Handle the HomeCarouselItem "deleted" event.
     *
     * @param  \App\Models\HomeCarouselItem  $homeCarouselItem
     * @return void
     */
    public function deleted(HomeCarouselItem $homeCarouselItem)
    {
        CacheService::clearCarousels();
    }

    /**
     * Handle the HomeCarouselItem "restored" event.
     *
     * @param  \App\Models\HomeCarouselItem  $homeCarouselItem
     * @return void
     */
    public function restored(HomeCarouselItem $homeCarouselItem)
    {
        CacheService::clearCarousels();
    }

    /**
     * Handle the HomeCarouselItem "force deleted" event.
     *
     * @param  \App\Models\HomeCarouselItem  $homeCarouselItem
     * @return void
     */
    public function forceDeleted(HomeCarouselItem $homeCarouselItem)
    {
        CacheService::clearCarousels();
    }
}
