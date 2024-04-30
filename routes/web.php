<?php

use App\Http\Controllers\Profile\ProfileController;
use App\Http\Controllers\SocialiteController;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\HomeController;
use App\Http\Controllers\Post\PostController;
use App\Http\Controllers\Post\GameController;

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


// Route::get('/test', function () {
//     return view('sample');
// });

Auth::routes([
    'login' => true,
    'logout' => true,
    'register' => true,
    'reset' => false,
    'verify' => false,
    'confirm' => false,
]);

Route::get('/lang/{locale}', [HomeController::class, 'lang'])->name('lang');


Route::get('/', [HomeController::class, 'index'])->name('home');

// short url
Route::get('g/{post:serial}', [GameController::class, 'show'])->name('game.show.short');
Route::get('r/{post:serial}', [GameController::class, 'rank'])->name('game.rank.short');

Route::get('post/{post:serial}/game', fn() => redirect()->route('game.show.short', ['post' => request()->post]))->name('game.show');;
Route::get('post/{post:serial}/rank', fn() => redirect()->route('game.rank.short', ['post' => request()->post]))->name('game.rank');
Route::get('post/{post:serial}/rank-embed', [GameController::class, 'rankEmbed'])->name('game.rank-embed');

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
Route::get('tos', fn() => view_or("tos.".app()->getLocale(), 'tos.en'))->name('tos');
Route::get('privacy', fn() => view_or("privacy.".app()->getLocale(), 'privacy.en'))->name('privacy');


