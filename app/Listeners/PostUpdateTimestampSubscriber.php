<?php

namespace App\Listeners;

use App\Events\PostDeleted;
use App\Events\PostUpdated;
use App\Helper\CacheService;
use App\Models\PublicPost;
use Illuminate\Events\Dispatcher;
use App\Events\PostCreated;

class PostUpdateTimestampSubscriber
{

    public function onPostCreated(PostCreated $event): void
    {
        logger('[PostUpdateTimestampSubscriber] onPostCreated', ['post' => $event->getPost()->id]);
        CacheService::rememberPostUpdatedTimestamp(true);
        CacheService::pullPublicPostFreshCache();
    }

    public function onPostDeleted(PostDeleted $event): void
    {
        logger('[PostUpdateTimestampSubscriber] onPostDeleted', ['post' => $event->getPost()->id]);
        CacheService::rememberPostUpdatedTimestamp(true);
        CacheService::pullPublicPostFreshCache();
    }

    public function onPostUpdated(PostUpdated $event): void
    {
        $post = $event->getPost();
        logger('[PostUpdateTimestampSubscriber] onPostUpdated', ['post' => $post->id]);

        if ($post->wasChanged('is_censored')) {
            if ($post->is_censored) {
                $publicPost = PublicPost::where('post_id', $post->id)->first();
                if ($publicPost) {
                    $publicPost->data = array_replace($publicPost->data ?? [], ['is_censored' => true]);
                    $publicPost->save();
                }
            }

            CacheService::pullPostResourceByPostId($post->id);
        }

        CacheService::rememberPostUpdatedTimestamp(true);
        CacheService::pullPublicPostFreshCache();
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
