<?php


namespace App\ScheduleExecutor;

use App\Enums\PostAccessPolicy;
use App\Enums\TrendTimeRange;
use App\Enums\TrendType;
use App\Helper\CacheService;
use App\Http\Resources\PostResource;
use App\Jobs\UpdatePostTrendsPosition;
use App\Models\Post;
use App\Models\PostTrend;
use App\Models\PublicPost;
use DB;

class PublicPostScheduleExecutor
{
    public function updatePublicPosts()
    {
        if (CacheService::hasPublicPostFreshCache()) {
            return;
        }

        try {
            DB::beginTransaction();
            $this->updateNewPublicPosts();
            DB::commit();
        } catch (\Exception $e) {
            DB::rollBack();
            report($e);
        }

        try {
            DB::beginTransaction();
            $this->updateTodayPublicPosts();
            DB::commit();
        } catch (\Exception $e) {
            DB::rollBack();
            report($e);
        }

        try {
            DB::beginTransaction();
            $this->updateWeekPublicPosts();
            DB::commit();
        } catch (\Exception $e) {
            DB::rollBack();
            report($e);
        }

        try {
            DB::beginTransaction();
            $this->updateMonthPublicPosts();
            DB::commit();
            CacheService::putPublicPostFreshCache();
        } catch (\Exception $e) {
            DB::rollBack();
            report($e);
        }

    }

    protected function updateNewPublicPosts()
    {
        PublicPost::getQuery()->update([
            'new_position' => 9999
        ]);

        $counter = 0;
        Post::whereRelation('post_policy','access_policy','=',PostAccessPolicy::PUBLIC)
            ->whereHas('elements', null, '>=', config('setting.post_min_element_count'))
            ->orderBy('id', 'desc')
            ->limit(1000)
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
                    'tags' => $post->tags->pluck('name')->implode(','),
                    'data' => PostResource::make($post)->toArray(request())
                ]);
            });
    }

    protected function updateTodayPublicPosts()
    {
        PublicPost::getQuery()->update([
            'day_position' => 9999
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
            ->limit(1000)
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
                    'tags' => $post->tags->pluck('name')->implode(','),
                    'data' => PostResource::make($post)->toArray(request())
                ]);
            });
    }

    protected function updateWeekPublicPosts()
    {
        PublicPost::getQuery()->update([
            'week_position' => 9999
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
            ->limit(1000)
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
                    'tags' => $post->tags->pluck('name')->implode(','),
                    'data' => PostResource::make($post)->toArray(request())
                ]);
            });
    }

    protected function updateMonthPublicPosts()
    {
        PublicPost::getQuery()->update([
            'month_position' => 9999
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
            ->limit(1000)
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
                    'tags' => $post->tags->pluck('name')->implode(','),
                    'data' => PostResource::make($post)->toArray(request())
                ]);
            });
    }
}
