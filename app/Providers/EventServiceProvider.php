<?php

namespace App\Providers;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Listeners\CreateGameResult;
use App\Listeners\DeleteElementRank;
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
            CreateImgurAlbum::class
        ],
        PostDeleted::class => [
            DeleteImgurAlbum::class
        ],
        ImageElementCreated::class => [
            CreateImgurImage::class
        ],
        ElementDeleted::class => [
            DeleteElementRank::class
        ]
    ];

    /**
     * Register any events for your application.
     *
     * @return void
     */
    public function boot()
    {
        //
    }
}
