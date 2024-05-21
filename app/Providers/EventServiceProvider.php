<?php

namespace App\Providers;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Events\VideoElementCreated;
use App\Listeners\CreateGameResult;
use App\Listeners\DeleteElementRank;
use App\Listeners\MakeVideoThumbnail;
use App\Listeners\UpdateElementRank;
use App\Listeners\UpdatePostRank;
use App\Events\ElementDeleted;
use App\Events\PostCreated;
use App\Events\PostDeleted;
use App\Listeners\CreateImgurAlbum;
use App\Listeners\CreateImgurImage;
use App\Listeners\DeleteImgurImage;
use App\Events\ImageElementCreated;
use App\Listeners\DeleteImgurAlbum;
use App\Models\HomeCarouselItem;
use Illuminate\Auth\Events\Registered;
use Illuminate\Auth\Listeners\SendEmailVerificationNotification;
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
        GameComplete::class => [
            CreateGameResult::class,
            UpdatePostRank::class
        ],
        GameElementVoted::class => [
            UpdateElementRank::class
        ],
        PostCreated::class => [
            //create a job to create imgur album
        ],
        PostDeleted::class => [
            //create a job to delete imgur album
        ],
        ImageElementCreated::class => [
            //create a job to upload image to imgur
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
