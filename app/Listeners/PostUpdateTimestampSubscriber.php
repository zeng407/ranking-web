<?php

namespace App\Listeners;

use App\Events\PostDeleted;
use App\Events\PostUpdated;
use App\Helper\CacheService;
use Illuminate\Events\Dispatcher;
use App\Events\PostCreated;

class PostUpdateTimestampSubscriber
{

    public function onPostCreated(PostCreated $event): void
    {
        logger('[PostUpdateTimestampSubscriber] onPostCreated', ['post' => $event->getPost()->id]);
        CacheService::rememberPostUpdatedTimestamp(true);
    }

    public function onPostDeleted(PostDeleted $event): void
    {
        logger('[PostUpdateTimestampSubscriber] onPostDeleted', ['post' => $event->getPost()->id]);
        CacheService::rememberPostUpdatedTimestamp(true);
    }

    public function onPostUpdated(PostUpdated $event): void
    {
        logger('[PostUpdateTimestampSubscriber] onPostUpdated', ['post' => $event->getPost()->id]);
        CacheService::rememberPostUpdatedTimestamp(true);
    }

    public function subscribe(Dispatcher $events): void
    {
        $events->listen(
            PostCreated::class,
            static::class . '@onPostCreated'
        );

        $events->listen(
            PostDeleted::class,
            static::class . '@onPostDeleted'
        );

        $events->listen(
            PostUpdated::class,
            static::class . '@onPostUpdated'
        );
    }
}
