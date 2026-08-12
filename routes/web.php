<?php

use App\Http\Controllers\Profile\ProfileController;
use App\Http\Controllers\SocialiteController;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\HomeController;
use App\Http\Controllers\Post\AdController;
use App\Http\Controllers\Post\PostController;
use App\Http\Controllers\Post\GameController;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

/*
|--------------------------------------------------------------------------
| Web Routes
|--------------------------------------------------------------------------
|
| Here is where you can register web routes for your application. These
| routes are loaded by the RouteServiceProvider within a group which
| contains the "web" middleware group. Now create something great!
|
*/


Auth::routes([
    'login' => true,
    'logout' => true,
    'register' => true,
    'reset' => true,
    'verify' => false,
    'confirm' => false,
]);

Route::get('/lang/{legacyLocale}/{path?}', function (Request $request, string $legacyLocale, string $path = '') {
    $localeMap = [
        'zh_TW' => 'zh-tw',
        'en' => 'en',
        'ja' => 'ja',
    ];

    abort_unless(isset($localeMap[$legacyLocale]), 404);

    $target = '/' . $localeMap[$legacyLocale] . '/' . ltrim($path, '/');
    if ($query = $request->getQueryString()) {
        $target .= '?' . $query;
    }

    return new RedirectResponse($target, 301);
})->where('path', '.*')->name('locale.legacy');

Route::get('/', [HomeController::class, 'index'])
    ->middleware(['locale.default', 'cache.public-html:home'])
    ->name('home');
Route::redirect('/donate', '/zh-tw/donate', 301)->name('donate');

// short url
Route::get('g/{post:serial}', [GameController::class, 'show'])->name('game.show');;
Route::get('r/{post:serial}', [GameController::class, 'rank'])
    ->middleware('cache.public-html:rank')
    ->name('game.rank');
Route::get('r/{post:serial}/access', [GameController::class, 'accessRank'])->name('game.rank-access');
Route::get('r/{post:serial}/embed', [GameController::class, 'rankEmbed'])->name('game.rank-embed');
Route::get('r/{post:serial}/export', [GameController::class, 'export'])->name('game.export');

// Image proxy for CORS
Route::get('/proxy-image', function () {
    if(!app()->environment('local')){
        abort(403, 'Image proxy is only available in local environment');
    }

    $url = request('url');
    if (!$url || !filter_var($url, FILTER_VALIDATE_URL)) {
        abort(400, 'Invalid URL');
    }

    // Only allow specific domains for security
    $allowedDomains = ['file.2pick.app', 'i.imgur.com'];
    $host = parse_url($url, PHP_URL_HOST);
    if (!in_array($host, $allowedDomains)) {
        abort(403, 'Domain not allowed');
    }

    try {
        $response = \Illuminate\Support\Facades\Http::timeout(10)->get($url);

        if (!$response->successful()) {
            abort(404, 'Image not found');
        }

        return response($response->body())
            ->header('Content-Type', $response->header('Content-Type'))
            ->header('Access-Control-Allow-Origin', '*')
            ->header('Cache-Control', 'public, max-age=86400');
    } catch (\Exception $e) {
        abort(500, 'Failed to fetch image');
    }
})->name('proxy.image');


// old url
Route::get('post/{post:serial}/game', fn() => redirect()->route('game.show', ['post' => request()->post]));
Route::get('post/{post:serial}/rank', fn() => redirect()->route('game.rank', ['post' => request()->post]));
Route::get('post/{post:serial}/rank-embed', fn() => redirect()->route('game.rank-embed', ['post' => request()->post]));

/** Oauth */

Route::get('/auth/google/redirect', [SocialiteController::class, 'redirectGoogle'])->name('auth.redirect.google');
Route::get('/auth/google/connect', [SocialiteController::class, 'connectGoogle'])->name('auth.connect.google');
Route::get('/auth/google/callback', [SocialiteController::class, 'callbackGoogle'])->name('auth.callback.google');
Route::get('/auth/twitch/callback', function(){
    return "Twitch callback";
})->name('auth.callback.twitch');

Route::middleware('auth')->group(function () {
    Route::get('account/profile', [ProfileController::class, 'index'])->name('profile.index');
    Route::put('account/profile', [ProfileController::class, 'update'])->name('profile.update');
    Route::put('account/password', [ProfileController::class, 'updatePassword'])->name('profile.update.password');
    Route::put('account/password/init', [ProfileController::class, 'initPassword'])->name('profile.update.password.init');
    Route::get('account/post', [PostController::class, 'index'])->name('post.index');
    Route::get('account/post/{post:serial}/edit', [PostController::class, 'edit'])->name('post.edit');
});

/** TOS */
Route::redirect('tos', '/zh-tw/tos', 301)->name('tos');
Route::redirect('privacy', '/zh-tw/privacy', 301)->name('privacy');

/** Game Room */
Route::get('b/{gameRoom:serial}', [GameController::class, 'joinRoom'])->name('game.room.index');
Route::get('/onead-media', [AdController::class, 'onead_media'])->name('onead.media');
