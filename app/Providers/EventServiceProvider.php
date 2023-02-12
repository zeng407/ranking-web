<?php

namespace App\Providers;

use App\Events\GameElementVoted;
use App\Events\GameComplete;
use App\Listeners\UpdateElementRank;
use App\Listeners\UpdatePostRank;
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
            UpdatePostRank::class
        ],
        GameElementVoted::class => [
            UpdateElementRank::class
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
