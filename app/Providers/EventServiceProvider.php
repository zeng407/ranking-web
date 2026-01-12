<?php

namespace App\Providers;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Events\RefreshGameCandidates;
use App\Events\VideoElementCreated;
use App\Listeners\CreateGameResult;
use App\Listeners\DeleteElementRank;
use App\Listeners\DeletePostRank;
use App\Listeners\DettachGameElements;
use App\Listeners\MakeVideoThumbnail;
use App\Listeners\NotifyGameRoomRefresh;
use App\Listeners\NotifyVoted;
use App\Listeners\UpdateElementRank;
use App\Listeners\UpdatePostRank;
use App\Events\ElementDeleted;
use App\Events\PostCreated;
use App\Events\PostDeleted;
use App\Events\ImageElementCreated;
use App\Listeners\DeleteElements;
use App\Models\HomeCarouselItem;
use Illuminate\Foundation\Support\Providers\EventServiceProvider as ServiceProvider;
use Illuminate\Support\Facades\Event;

class EventServiceProvider extends ServiceProvider
{
    /**
     * The event listener mappings for the application.
     *
     * @var array
     */
    protected $listen = [
//        Registered::class => [
//            SendEmailVerificationNotification::class,
//        ],
        RefreshGameCandidates::class => [
            NotifyGameRoomRefresh::class
        ],
        GameComplete::class => [
            CreateGameResult::class,
            UpdatePostRank::class,
            DettachGameElements::class,
        ],
        GameElementVoted::class => [
            UpdateElementRank::class,
            NotifyVoted::class,
        ],
        PostCreated::class => [

        ],
        PostDeleted::class => [
            DeletePostRank::class,
            DeleteElements::class
        ],
        ImageElementCreated::class => [
        ],
        VideoElementCreated::class => [
            MakeVideoThumbnail::class
        ],
        ElementDeleted::class => [
            DeleteElementRank::class
        ],
        \SocialiteProviders\Manager\SocialiteWasCalled::class => [
            // ... other providers
            \SocialiteProviders\Google\GoogleExtendSocialite::class.'@handle',
        ],
    ];

    /**
     * Register any events for your application.
     *
     * @return void
     */
    public function boot()
    {
        Event::subscribe(\App\Listeners\PostUpdateTimestampSubscriber::class);
        HomeCarouselItem::observe(\App\Observers\HomeCarouselItemObserver::class);
    }
}
