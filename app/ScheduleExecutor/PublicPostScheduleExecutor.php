<?php


namespace App\ScheduleExecutor;

use App\Enums\PostAccessPolicy;
use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
use App\Models\Post;
use App\Models\PostTrend;
use App\Models\PublicPost;

class PublicPostScheduleExecutor
{
    const POST_BATCH_SIZE = 2000;
    public function updatePublicPosts()
    {
        if (CacheService::hasPublicPostFreshCache()) {
            return;
        }

        try {
            $this->updateNewPublicPosts();
        } catch (\Exception $e) {
            report($e);
        }

        try {
            $this->updateTodayPublicPosts();
        } catch (\Exception $e) {
            report($e);
        }

        try {
            $this->updateWeekPublicPosts();
        } catch (\Exception $e) {
            report($e);
        }

        try {
            $this->updateMonthPublicPosts();
            $this->removeDirtyPublicPosts();
            CacheService::putPublicPostFreshCache();
        } catch (\Exception $e) {
            report($e);
        }

        try {
            $this->clearPostResourceCache();
        } catch (\Exception $e) {
            report($e);
        }

    }

    protected function updateNewPublicPosts()
    {
        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $counter = 0;
        Post::whereRelation('post_policy','access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('elements', null, '>=', config('setting.post_min_element_count'))
            ->orderBy('id', 'desc')
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function ($post) use (&$counter) {
                $counter++;
                PublicPost::updateOrCreate([
                    'post_id' => $post->id,
                ], [
                    'post_id' => $post->id,
                    'new_position' => $counter,
                    'title' => $post->title,
                    'description' => $post->description,
                    'tags' => json_encode($post->tags->pluck('name')->toArray(), JSON_UNESCAPED_UNICODE),
                    'data' => CacheService::rememberPostResource($post),
                    'is_dirty' => false,
                ]);
            });

        PublicPost::getQuery()
            ->where('is_dirty', true)
            ->update([
                'new_position' => 9999,
            ]);

    }

    protected function updateTodayPublicPosts()
    {
        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $counter = 0;
        $startDate = today()->toDateString();
        PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::TODAY)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;
                $counter++;
                PublicPost::updateOrCreate([
                    'post_id' => $post->id,
                ], [
                    'post_id' => $post->id,
                    'day_position' => $counter,
                    'title' => $post->title,
                    'description' => $post->description,
                    'tags' => json_encode($post->tags->pluck('name')->toArray(), JSON_UNESCAPED_UNICODE),
                    'data' => CacheService::rememberPostResource($post),
                    'is_dirty' => false,
                ]);
            });

        PublicPost::getQuery()
            ->where('is_dirty', true)
            ->update([
                'day_position' => 9999,
            ]);
    }

    protected function updateWeekPublicPosts()
    {
        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $counter = 0;
        $startDate = today()->startOfWeek()->toDateString();
        PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::WEEK)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;
                $counter++;
                PublicPost::updateOrCreate([
                    'post_id' => $post->id,
                ], [
                    'post_id' => $post->id,
                    'week_position' => $counter,
                    'title' => $post->title,
                    'description' => $post->description,
                    'tags' => json_encode($post->tags->pluck('name')->toArray(), JSON_UNESCAPED_UNICODE),
                    'data' => CacheService::rememberPostResource($post),
                    'is_dirty' => false,
                ]);
            });

        PublicPost::getQuery()
            ->where('is_dirty', true)
            ->update([
                'week_position' => 9999,
            ]);
    }

    protected function updateMonthPublicPosts()
    {
        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $counter = 0;
        $startDate = today()->startOfMonth()->toDateString();
        PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::MONTH)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;
                $counter++;
                PublicPost::updateOrCreate([
                    'post_id' => $post->id,
                ], [
                    'post_id' => $post->id,
                    'month_position' => $counter,
                    'title' => $post->title,
                    'description' => $post->description,
                    'tags' => json_encode($post->tags->pluck('name')->toArray(), JSON_UNESCAPED_UNICODE),
                    'data' => CacheService::rememberPostResource($post),
                    'is_dirty' => false,
                ]);
            });

        PublicPost::getQuery()
            ->where('is_dirty', true)
            ->update([
                'month_position' => 9999,
            ]);
    }

    protected function removeDirtyPublicPosts()
    {
        PublicPost::where('is_dirty', true)
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function (PublicPost $publicPost) {
                if(!$publicPost->post) {
                    $publicPost->delete();
                }else if(!$publicPost->post->isPublic()) {
                    $publicPost->delete();
                }else if($publicPost->post->elements()->count() < config('setting.post_min_element_count')) {
                    $publicPost->delete();
                }
            });
    }

    protected function clearPostResourceCache()
    {
        PublicPost::select('post_id')
            ->each(function ($publicPost) {
                CacheService::pullPostResourceByPostId($publicPost->post_id);
            });
    }
}
