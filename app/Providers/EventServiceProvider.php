<?php

namespace App\Providers;

use App\Events\GameComplete;
use App\Events\ElementDeleted;
use App\Events\PostCreated;
use App\Events\PostDeleted;
use App\Listeners\CreateImgurAlbum;
use App\Listeners\CreateImgurImage;
use App\Listeners\DeleteImgurImage;
use App\Listeners\UpdatePostRank;
use Illuminate\Foundation\Support\Providers\EventServiceProvider as ServiceProvider;
use App\Events\ImageElementCreated;
use App\Listeners\DeleteImgurAlbum;

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
            UpdatePostRank::class
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
            DeleteImgurImage::class
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
