<?php


namespace App\ScheduleExecutor;

use App\Enums\PostAccessPolicy;
use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
use App\Models\Post;
use App\Models\PostTrend;
use App\Models\PublicPost;
use Illuminate\Support\Facades\Log;

class PublicPostScheduleExecutor
{
    const POST_BATCH_SIZE = 2000;
    public function updatePublicPosts()
    {
        $started = microtime(true);

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

        Log::info('updatePublicPosts finished', [
            'duration_ms' => round((microtime(true) - $started) * 1000, 2),
        ]);

    }

    protected function updateNewPublicPosts()
    {
        $started = microtime(true);

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

        Log::info('updateNewPublicPosts finished', [
            'processed' => $counter,
            'duration_ms' => round((microtime(true) - $started) * 1000, 2),
        ]);
    }

    protected function updateTodayPublicPosts()
    {
        $startedOverall = microtime(true);
        $counter = 0;
        $startDate = today()->toDateString();
        $started = microtime(true);

        $trends = PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::TODAY)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get();

        $fetchDurationMs = round((microtime(true) - $started) * 1000, 2);
        Log::info('updateTodayPublicPosts fetch completed', [
            'start_date' => $startDate,
            'fetched' => $trends->count(),
            'duration_ms' => $fetchDurationMs,
        ]);

        if ($trends->isEmpty()) {
            Log::info('updateTodayPublicPosts skipped (no trends)', [
                'start_date' => $startDate,
                'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
            ]);
            return;
        }

        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $trends
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;

                if (!$post) {
                    return;
                }

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

        Log::info('updateTodayPublicPosts finished', [
            'start_date' => $startDate,
            'processed' => $counter,
            'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
        ]);
    }

    protected function updateWeekPublicPosts()
    {
        $startedOverall = microtime(true);

        $counter = 0;
        $startDate = today()->startOfWeek()->toDateString();

        $fetchStarted = microtime(true);
        $trends = PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::WEEK)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get();

        $fetchDurationMs = round((microtime(true) - $fetchStarted) * 1000, 2);
        Log::info('updateWeekPublicPosts fetch completed', [
            'start_date' => $startDate,
            'fetched' => $trends->count(),
            'duration_ms' => $fetchDurationMs,
        ]);

        if ($trends->isEmpty()) {
            Log::info('updateWeekPublicPosts skipped (no trends)', [
                'start_date' => $startDate,
                'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
            ]);
            return;
        }

        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $trends
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;

                if (!$post) {
                    return;
                }

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

        Log::info('updateWeekPublicPosts finished', [
            'start_date' => $startDate,
            'processed' => $counter,
            'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
        ]);
    }

    protected function updateMonthPublicPosts()
    {
        $startedOverall = microtime(true);

        $counter = 0;
        $startDate = today()->startOfMonth()->toDateString();

        $fetchStarted = microtime(true);
        $trends = PostTrend::with(['post'])
            ->whereRelation('post', 'deleted_at', null)
            ->whereRelation('post.post_policy', 'access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('post.elements', null, '>=', config('setting.post_min_element_count'))
            ->where('trend_type', TrendType::HOT)
            ->where('time_range', TrendTimeRange::MONTH)
            ->where('start_date', $startDate)
            ->orderBy('position')
            ->limit(self::POST_BATCH_SIZE)
            ->get();

        $fetchDurationMs = round((microtime(true) - $fetchStarted) * 1000, 2);
        Log::info('updateMonthPublicPosts fetch completed', [
            'start_date' => $startDate,
            'fetched' => $trends->count(),
            'duration_ms' => $fetchDurationMs,
        ]);

        if ($trends->isEmpty()) {
            Log::info('updateMonthPublicPosts skipped (no trends)', [
                'start_date' => $startDate,
                'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
            ]);
            return;
        }

        PublicPost::getQuery()->update([
            'is_dirty' => true,
        ]);

        $trends
            ->each(function ($trend) use (&$counter) {
                $post = $trend->post;

                if (!$post) {
                    return;
                }

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

        Log::info('updateMonthPublicPosts finished', [
            'start_date' => $startDate,
            'processed' => $counter,
            'duration_ms' => round((microtime(true) - $startedOverall) * 1000, 2),
        ]);
    }

    protected function removeDirtyPublicPosts()
    {
        $started = microtime(true);
        $deleted = 0;

        PublicPost::where('is_dirty', true)
            ->limit(self::POST_BATCH_SIZE)
            ->get()
            ->each(function (PublicPost $publicPost) use (&$deleted) {
                if(!$publicPost->post) {
                    $publicPost->delete();
                    $deleted++;
                } else if(!$publicPost->post->isPublic()) {
                    $publicPost->delete();
                    $deleted++;
                } else if($publicPost->post->elements()->count() < config('setting.post_min_element_count')) {
                    $publicPost->delete();
                    $deleted++;
                }
            });

        Log::info('removeDirtyPublicPosts finished', [
            'deleted' => $deleted,
            'duration_ms' => round((microtime(true) - $started) * 1000, 2),
        ]);
    }

    protected function clearPostResourceCache()
    {
        $started = microtime(true);
        $count = 0;

        PublicPost::select('post_id')
            ->each(function ($publicPost) use (&$count) {
                CacheService::pullPostResourceByPostId($publicPost->post_id);
                $count++;
            });

        Log::info('clearPostResourceCache finished', [
            'processed' => $count,
            'duration_ms' => round((microtime(true) - $started) * 1000, 2),
        ]);
    }
}
