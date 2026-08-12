<?php

use App\Http\Controllers\Auth\LoginController;
use App\Http\Controllers\HomeController;
use App\Http\Controllers\Post\GameController;
use Illuminate\Support\Facades\Route;

/*
|--------------------------------------------------------------------------
| Localized public routes
|--------------------------------------------------------------------------
|
| Public URL locale segments are deliberately different from Laravel's
| translation directory names: zh-tw maps to zh_TW in LocalePrefixRoute.
| The URL is the source of truth; session locale is only a preference.
|
*/

Route::prefix('{locale}')
    ->where(['locale' => 'zh-tw|en|ja'])
    ->as('localized.')
    ->group(function () {
        Route::get('/', [HomeController::class, 'index'])
            ->middleware('cache.public-html:home')
            ->name('home');
        Route::get('/tos', fn() => view_or("tos." . app()->getLocale(), 'tos.en'))
            ->middleware('cache.public-html')
            ->name('tos');
        Route::get('/privacy', fn() => view_or("privacy." . app()->getLocale(), 'privacy.en'))
            ->middleware('cache.public-html')
            ->name('privacy');
        Route::get('/donate', [HomeController::class, 'donate'])
            ->middleware('cache.public-html')
            ->name('donate');
        Route::get('/login', [LoginController::class, 'showLoginForm'])->name('login');
        Route::get('/hot', [HomeController::class, 'hot'])->name('hot');
        Route::get('/new', [HomeController::class, 'new'])->name('new');
        Route::get('g/{post:serial}', [GameController::class, 'show'])->name('game.show');
        Route::get('r/{post:serial}', [GameController::class, 'rank'])
            ->middleware('cache.public-html:rank')
            ->name('game.rank');
        Route::get('post/{post:serial}/game', [GameController::class, 'show']);
        Route::get('post/{post:serial}/rank', [GameController::class, 'rank']);
    });
