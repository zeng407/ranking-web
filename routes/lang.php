<?php

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


Route::prefix('{locale}')->middleware('locale.prefix')->group(function () {
    Route::get('/', [HomeController::class, 'index']);
    Route::get('/hot', [HomeController::class, 'hot']);
    Route::get('/new', [HomeController::class, 'new']);
    Route::get('post/{post:serial}/game', [GameController::class, 'show']);
    Route::get('post/{post:serial}/rank', [GameController::class, 'rank']);
});