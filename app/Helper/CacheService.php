<?php

namespace App\Helper;

use App\Http\Resources\Game\GameRoomUserResource;
use App\Http\Resources\PublicPostResource;
use App\Models\GameRoom;
use App\Models\GameRoomUser;
use App\Models\User;
use App\Repositories\Filters\PostFilter;
use App\Services\HomeCarouselService;
use App\Services\PostService;
use App\Services\TagService;
use Cache;
use Illuminate\Database\Eloquent\Collection;
use Illuminate\Http\Request;


class CacheService
{
    static public function rememberAnnouncement($data = null, $minutes = 60, $refresh = false)
    {
        if ($refresh) {
            Cache::forget('announcement');
        }
        $seconds = $minutes * 60;
        return Cache::remember('announcement', $seconds, function() use ($data) {
            return $data;
        });
    }

    static public function rememberUserRole(User $user, $refresh = false)
    {
        if ($refresh) {
            Cache::forget('user_role_' . $user->id);
        }
        $seconds = 60 * 60;
        return Cache::remember('user_role_' . $user->id, $seconds, function() use ($user) {
            return $user->roles()->pluck('slug')->toArray();
        });
    }

    static public function rememberHotTags($refresh = false)
    {
        if ($refresh) {
            Cache::forget('hot_tags');
        }
        $seconds = 60 * 60; // 1 hour
        return Cache::remember('hot_tags', $seconds, function() {
            return app(TagService::class)->getHotTags();
        });
    }

    static public function rememberPosts(Request $request, $sort, $refresh = false)
    {
        $url = $request->getQueryString();
        $cacheName = Cache::get('post_update_at').'/'.md5($url);
        if ($refresh) {
            Cache::forget($cacheName);
        }
        $seconds = 60 * 5; // 5 minutes
        logger('cacheName: '.$cacheName . ' url: '.$url);
        $cache = Cache::remember($cacheName, $seconds , function() use ($request, $sort) {
            logger('cache miss');
            $posts = app(PostService::class)->getList([
                PostFilter::PUBLIC => true,
                PostFilter::ELEMENTS_COUNT_GTE => config('setting.post_min_element_count'),
                PostFilter::KEYWORD_LIKE => $request->query('k')
            ],[
                'sort_by' => $sort,
            ], [
                'per_page' => config('setting.home_post_per_page')
            ]);
            foreach($posts as $key => $post ) {
                $posts[$key] = PublicPostResource::make($post)->toArray($request);
            }
            return $posts;
        });

        return $cache;
    }


    static public function rememberPostUpdatedTimestamp($fresh = false)
    {
        if ($fresh) {
            Cache::forget('post_update_at');
        }
        $seconds = 60 * 60 ; // 1 hour
        return Cache::remember('post_update_at', $seconds, function() {
            return now()->unix().rand(0, 10000);
        });
    }

    static public function rememberCarousels($refresh = false)
    {
        if ($refresh) {
            self::clearCarousels();
        }
        $seconds = 60 * 60; // 1 hour
        return Cache::remember('carousels', $seconds, function() {
            return app(HomeCarouselService::class)->getHomeCarouselItems();
        });
    }

    static public function clearCarousels()
    {
        Cache::forget('carousels');
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
        if ($refresh) {
            Cache::forget('game_bet_rank'.$gameRoom->serial);
        }
        $seconds = 60 * 60; // 1 hour
        return Cache::remember('game_bet_rank'.$gameRoom->serial, $seconds, function()use($gameRoom){
            $mapData = function(Collection $collection){
                return $collection->map(function($gameUser) {
                    return GameRoomUserResource::make($gameUser)->toArray(request());
                });
            };
            $top10 = $mapData($gameRoom->users()->orderBy('rank')->limit(10)->get());
            $bottom10 = $mapData($gameRoom->users()->orderByDesc('rank')->limit(10)->get());
            return [
                'total_users' => $gameRoom->users()->count(),
                'top_10' => $top10,
                'bottom_10' => $bottom10
            ];
        });
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
}
