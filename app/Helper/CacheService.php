<?php

namespace App\Helper;

use App\Enums\PostAccessPolicy;
use App\Enums\RankReportTimeRange;
use App\Http\Resources\Game\ChampionResource;
use App\Http\Resources\Game\GameRoomUserResource;
use App\Http\Resources\PostResource;
use App\Http\Resources\PublicPostResource;
use App\Models\Game;
use App\Models\GameRoom;
use App\Models\GameRoomUser;
use App\Models\Post;
use App\Models\User;
use App\Models\UserGameResult;
use App\Repositories\Filters\PostFilter;
use App\Repositories\PublicPostRepository;
use App\Services\HomeCarouselService;
use App\Services\PostService;
use App\Services\PublicPostService;
use App\Services\SoketiService;
use App\Services\TagService;
use Cache;
use Illuminate\Database\Eloquent\Collection;
use Illuminate\Http\Request;


class CacheService
{
    static public function rememberAnnouncement($data = null, $minutes = 60, $refresh = false)
    {
        $key = 'announcement';
        $seconds = $minutes * 60;

        return static::remember($key, $seconds, function () use ($data) {
            return $data;
        }, $refresh);
    }

    static public function rememberUserRole(User $user, $refresh = false)
    {
        $key = 'user_role_' . $user->id;
        $seconds = 60 * 60;

        return static::remember($key, $seconds, function () use ($user) {
            return $user->roles()->pluck('slug')->toArray();
        }, $refresh);
    }

    static public function rememberHotTags($limit = 10, $refresh = false)
    {
        $key = 'hot_tags';
        $seconds = 60 * 60;
        return static::remember($key, $seconds, function () use ($limit) {
            return app(TagService::class)->getHotTags($limit);
        }, $refresh);
    }

    static public function rememberTags(string $keywork = '', $refresh = false)
    {
        $key = 'post_tags_' . $keywork;
        $seconds = 10 * 60; // 10 minutes
        return static::remember($key, $seconds, function () use ($keywork) {
            return app(TagService::class)->get($keywork);
        }, $refresh);
    }

    static public function rememberPosts(Request $request, $sort, $refresh = false)
    {
        $url = $request->getQueryString();
        $cacheName = Cache::get('post_update_at') . '/' . md5($url);
        if ($request->query('sort_by', 'hot') === 'hot') {
            $seconds = 60 * 60; // 1 hour
        } else {
            $seconds = 60 * 5; // 5 minutes
        }
        return static::remember($cacheName, $seconds, function () use ($request, $sort) {
            $posts = app(PublicPostService::class)->getList([
                PostFilter::KEYWORD_LIKE => $request->query('k')
            ], [
                'sort_by' => $sort,
            ], [
                'per_page' => config('setting.home_post_per_page')
            ]);
            foreach ($posts as $key => $post) {
                $posts[$key] = PublicPostResource::make($post)->toArray($request);
            }
            return $posts;
        }, $refresh);
    }


    static public function rememberPostUpdatedTimestamp($fresh = false)
    {
        $key = 'post_update_at';
        $seconds = 60 * 60; // 1 hour
        return static::remember($key, $seconds, function () {
            return now()->unix() . rand(0, 10000);
        }, $fresh);
    }

    static public function rememberCarousels($refresh = false)
    {
        $key = 'carousels';
        $seconds = 60 * 60; // 1 hour
        return static::remember($key, $seconds, function () {
            return app(HomeCarouselService::class)->getHomeCarouselItems();
        }, $refresh);
    }

    static public function rememberPostResource(Post $post)
    {
        $key = 'post_resource_' . $post->id;
        $seconds = 60 * 60; // 1 hour
        return static::remember($key, $seconds, function () use ($post) {
            return PostResource::make($post)->toArray(request());
        }, false);
    }

    static public function pullPostResourceByPostId($postId)
    {
        $key = 'post_resource_' . $postId;
        return Cache::pull($key);
    }

    static public function clearCarousels()
    {
        Cache::forget('carousels');
    }

    static public function hasPublicPostFreshCache()
    {
        return Cache::has('public_post_fresh');
    }

    static public function putPublicPostFreshCache()
    {
        Cache::put('public_post_fresh', true, 60 * 10);
    }

    static public function pullPublicPostFreshCache()
    {
        return Cache::pull('public_post_fresh');
    }

    static public function putUpdatingGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::put('processing_job:update_game_room_rank' . $gameRoom->serial, true, 60 * 24);
    }

    static public function pullUpdatingGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::pull('processing_job:update_game_room_rank' . $gameRoom->serial);
    }

    static public function hasUpdatingGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::has('processing_job:update_game_room_rank' . $gameRoom->serial);
    }

    static public function putJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::put('waiting_job:update_game_room_rank' . $gameRoom->serial, true, 60 * 24);
    }

    static public function pullJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::pull('waiting_job:update_game_room_rank' . $gameRoom->serial);
    }

    static public function hasJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::has('waiting_job:update_game_room_rank' . $gameRoom->serial);
    }

    static public function pullJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        return Cache::pull('waiting_job:update_game_bet' . $gameRoom->serial);
    }

    static public function putJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        Cache::put('waiting_job:update_game_bet' . $gameRoom->serial, true, 60 * 24);
    }

    static public function hasJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        return Cache::has('waiting_job:update_game_bet' . $gameRoom->serial);
    }

    static public function rememberGameBetRank(GameRoom $gameRoom, $refresh = false)
    {
        $key = 'game_bet_rank' . $gameRoom->serial;
        $seconds = 60; // 1 minute
        return static::remember($key, $seconds, function () use ($gameRoom) {
            $mapData = function (Collection $collection) {
                return $collection->map(function ($gameUser) {
                    return GameRoomUserResource::make($gameUser)->toArray(request());
                });
            };
            $top10 = $mapData($gameRoom->users()->where('rank', '>', 0)->orderBy('rank')->limit(10)->get());
            $bottom10 = $mapData($gameRoom->users()->where('rank', '>', 0)->orderByDesc('rank')->limit(10)->get());
            return [
                'total_users' => $gameRoom->users()->count(),
                'top_10' => $top10,
                'bottom_10' => $bottom10
            ];
        }, $refresh);
    }

    static public function rememberGameResult(Game $game, callable $callback, $refresh = false)
    {
        $updatedAt = $game->updated_at?->getTimestamp() ?? $game->id;
        $key = 'game_result:' . $game->id . ':' . $updatedAt;
        $seconds = 30 * 60; // 30 minutes
        return static::remember($key, $seconds, $callback, $refresh);
    }

    static public function rememberRankReports(Post $post, $refresh = false)
    {
        $key = 'rank_reports_all:' . $post->id;
        $seconds = 10 * 60; // 10 minutes
        return static::remember($key, $seconds, function () use ($post) {
            return \DB::table('rank_reports')
                ->join('elements', 'rank_reports.element_id', '=', 'elements.id')
                ->where('rank_reports.post_id', $post->id)
                ->whereNull('elements.deleted_at')
                ->orderByRaw('ISNULL(rank_reports.rank)')
                ->orderBy('rank_reports.rank')
                ->select('rank_reports.id', 'rank_reports.element_id', 'rank_reports.rank', 'rank_reports.win_rate')
                ->get()
                ->map(function ($item) {
                    return [
                        'id' => $item->id,
                        'element_id' => $item->element_id,
                        'rank' => $item->rank,
                        'win_rate' => $item->win_rate,
                    ];
                })
                ->toArray();
        }, $refresh);
    }

    static function putUpdateGameUserNameThreashold(GameRoomUser $gameRoomUser)
    {
        $time = 60 * 60; // 1 hour
        Cache::put('update_game_user_name_threashold' . $gameRoomUser->id, true, $time);
    }

    static function hasUpdateGameUserNameThreashold(GameRoomUser $gameRoomUser)
    {
        return Cache::has('update_game_user_name_threashold' . $gameRoomUser->id);
    }

    static function rememebrChannelSubscriptionCount($channel, $refresh = false)
    {
        $key = 'channel_subscription_count' . $channel;
        $seconds = 30;
        return static::remember($key, $seconds, function () use ($channel) {
            return app(SoketiService::class)->getSubscriptionCount($channel);
        }, $refresh);
    }

    static function addChampion(UserGameResult $result)
    {
        $key = 'champion_queue';
        $seconds = 3 * 60 * 60; // 3 hour
        $games = Cache::get($key);
        if ($games) {
            $games = collect($games);
            $games->prepend($result);
            $games = $games->take(5);
        } else {
            $games = collect([$result]);
        }

        Cache::put($key, $games, $seconds);
    }

    static function rememberChampions($refresh = false)
    {
        $key = 'champion';
        $seconds = 60 * 10; // 10 minute
        return static::remember($key, $seconds, function () {
            $champions = Cache::get('champion_queue');
            if ($champions) {
                return ChampionResource::collection($champions)->toArray(request());
            }
            return [];
        }, $refresh);
    }

    static function remember($key, $seconds, callable $callback, $refresh)
    {
        if ($refresh) {
            $data = $callback();
            Cache::forget($key);
            return Cache::remember($key, $seconds, function () use ($data) {
                return $data;
            });
        }
        return Cache::remember($key, $seconds, function () use ($callback) {
            return $callback();
        });
    }

    static function getRankHistoryJobCache($postId)
    {
        return Cache::get('CreateAndUpdateRankHistory:' . $postId);
    }

    static function putRankHistoryJobCache($postId, $count)
    {
        Cache::put('CreateAndUpdateRankHistory:' . $postId, $count, now()->addMinutes(10));
    }

    static function putRankHistoryNeededUpdateDatesCache($postId, RankReportTimeRange $timeRange, string $date)
    {
        $previous = Cache::pull('RankHistoryNeededUpdateDatesCache:' . $postId . '_' . $timeRange->value);
        $dates = $previous ? array_merge((array) $previous, (array) $date) : (array) $date;
        $dates = array_unique($dates);
        Cache::put('RankHistoryNeededUpdateDatesCache:' . $postId . '_' . $timeRange->value, $dates, now()->addDays(30));
    }

    static function pullRankHistoryNeededUpdateDatesCache($postId, RankReportTimeRange $timeRange)
    {
        return Cache::pull('RankHistoryNeededUpdateDatesCache:' . $postId . '_' . $timeRange->value);
    }

    static function setNeedFreshPostRank(Post $post)
    {
        $key = 'need_fresh_post_rank_' . $post->id;
        $seconds = 3 * 24 * 60 * 60; // 3 days
        Cache::put($key, true, $seconds);
    }

    static function getNeedFreshPostRank(Post $post)
    {
        $key = 'need_fresh_post_rank_' . $post->id;
        return Cache::get($key, false);
    }

    static function clearNeedFreshPostRank(Post $post)
    {
        $key = 'need_fresh_post_rank_' . $post->id;
        Cache::forget($key);
    }

    static function isSkipAds()
    {
        $anonymousId = session()->get('anonymous_id', 'unknown');
        $key = 'skip_ads_' . $anonymousId;
        return Cache::has($key);
    }

    static function setSkipAds()
    {
        $anonymousId = session()->get('anonymous_id', 'unknown');
        $key = 'skip_ads_' . $anonymousId;
        $seconds = 60 * 60 * 24; // 1 day
        Cache::put($key, true, $seconds);
    }
}
