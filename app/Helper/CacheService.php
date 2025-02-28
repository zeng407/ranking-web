<?php

namespace App\Helper;

use App\Enums\PostAccessPolicy;
use App\Http\Resources\Game\ChampionResource;
use App\Http\Resources\Game\GameRoomUserResource;
use App\Http\Resources\PostResource;
use App\Http\Resources\PublicPostResource;
use App\Models\Game;
use App\Models\GameRoom;
use App\Models\GameRoomUser;
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

        return static::remember($key, $seconds, function() use ($data) {
            return $data;
        }, $refresh);
    }

    static public function rememberUserRole(User $user, $refresh = false)
    {
        $key = 'user_role_' . $user->id;
        $seconds = 60 * 60;

        return static::remember($key, $seconds, function() use ($user) {
            return $user->roles()->pluck('slug')->toArray();
        }, $refresh);
    }

    static public function rememberHotTags($limit = 10, $refresh = false)
    {
        $key = 'hot_tags';
        $seconds = 60 * 60;
        return static::remember($key, $seconds, function() use ($limit) {
            return app(TagService::class)->getHotTags($limit);
        }, $refresh);
    }

    static public function rememberTags(string $keywork = '', $refresh = false)
    {
        $key = 'post_tags_'.$keywork;
        $seconds = 10 * 60; // 10 minutes
        return static::remember($key, $seconds, function() use ($keywork) {
            return app(TagService::class)->get($keywork);
        }, $refresh);

    }

    static public function rememberPosts(Request $request, $sort, $refresh = false)
    {
        $url = $request->getQueryString();
        $cacheName = Cache::get('post_update_at').'/'.md5($url);
        $seconds = 60 * 5; // 5 minutes
        return static::remember($cacheName, $seconds, function() use ($request, $sort) {
            $posts = app(PublicPostService::class)->getList([
                PostFilter::KEYWORD_LIKE => $request->query('k')
            ],[
                'sort_by' => $sort,
            ], [
                'per_page' => config('setting.home_post_per_page')
            ]);
            foreach($posts as $key => $post) {
                $posts[$key] = PublicPostResource::make($post)->toArray($request);
            }
            return $posts;
        }, $refresh);
    }


    static public function rememberPostUpdatedTimestamp($fresh = false)
    {
        $key = 'post_update_at';
        $seconds = 60 * 60 ; // 1 hour
        return static::remember($key, $seconds, function() {
            return now()->unix().rand(0, 10000);
        }, $fresh);
    }

    static public function rememberCarousels($refresh = false)
    {
        $key = 'carousels';
        $seconds = 60 * 60; // 1 hour
        return static::remember($key, $seconds, function() {
            return app(HomeCarouselService::class)->getHomeCarouselItems();
        }, $refresh);
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

    static public function putJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        Cache::put('waiting_job:update_game_room_rank'.$gameRoom->serial, true, 60 * 24);
    }

    static public function pullJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::pull('waiting_job:update_game_room_rank'.$gameRoom->serial);
    }

    static public function hasJobCacheUpdateGameRoomRank(GameRoom $gameRoom)
    {
        return Cache::has('waiting_job:update_game_room_rank'.$gameRoom->serial);
    }

    static public function pullJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        return Cache::pull('waiting_job:update_game_bet'.$gameRoom->serial);
    }

    static public function putJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        Cache::put('waiting_job:update_game_bet'.$gameRoom->serial, true, 60 * 24);
    }

    static public function hasJobCacheUpdateGameBet(GameRoom $gameRoom)
    {
        return Cache::has('waiting_job:update_game_bet'.$gameRoom->serial);
    }

    static public function rememberGameBetRank(GameRoom $gameRoom, $refresh = false)
    {
        $key = 'game_bet_rank'.$gameRoom->serial;
        $seconds = 60 ; // 1 minute
        return static::remember($key, $seconds, function() use ($gameRoom) {
            $mapData = function(Collection $collection){
                return $collection->map(function($gameUser) {
                    return GameRoomUserResource::make($gameUser)->toArray(request());
                });
            };
            $top10 = $mapData($gameRoom->users()->where('rank','>',0)->orderBy('rank')->limit(10)->get());
            $bottom10 = $mapData($gameRoom->users()->where('rank','>',0)->orderByDesc('rank')->limit(10)->get());
            return [
                'total_users' => $gameRoom->users()->count(),
                'top_10' => $top10,
                'bottom_10' => $bottom10
            ];
        }, $refresh);
    }

    static function putUpdateGameUserNameThreashold(GameRoomUser $gameRoomUser)
    {
        $time = 60 * 60; // 1 hour
        Cache::put('update_game_user_name_threashold'.$gameRoomUser->id, true, $time);
    }

    static function hasUpdateGameUserNameThreashold(GameRoomUser $gameRoomUser)
    {
        return Cache::has('update_game_user_name_threashold'.$gameRoomUser->id);
    }

    static function rememebrChannelSubscriptionCount($channel, $refresh = false)
    {
        $key = 'channel_subscription_count'.$channel;
        $seconds = 30;
        return static::remember($key, $seconds, function() use ($channel) {
            return app(SoketiService::class)->getSubscriptionCount($channel);
        }, $refresh);
    }

    static function rememberChampions($refresh = false)
    {
        $key = 'champion';
        $seconds = 60 * 10; // 10 minute
        return static::remember($key, $seconds, function() {
            $games = UserGameResult::with('game', 'champion', 'loser', 'game.post')
                ->whereHas('game.post', function ($query) {
                    $query->where('is_censored', false)
                        ->whereHas('post_policy', function ($query) {
                            $query->where('access_policy', PostAccessPolicy::PUBLIC);
                        });
                })
                ->orderByDesc('id')
                ->limit(5)
                ->get();

            return ChampionResource::collection($games);
        }, $refresh);
    }

    static function remember($key, $seconds, callable $callback, $refresh)
    {
        if ($refresh) {
            $data = $callback();
            Cache::forget($key);
            return Cache::remember($key, $seconds, function() use ($data) {
                return $data;
            });
        }
        return Cache::remember($key, $seconds, function() use ($callback) {
            return $callback();
        });
    }

}
