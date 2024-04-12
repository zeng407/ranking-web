<?php

use App\Http\Controllers\Auth\LoginController;
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



Route::prefix('lang/{locale}')->middleware('locale.prefix')->group(function () {
    $back = fn() => redirect()->back();
    Route::get('/', $back);
    Route::get('/login', $back);
    Route::get('/hot', $back);
    Route::get('/new', $back);
    Route::get('post/{post:serial}/game', $back);
    Route::get('post/{post:serial}/rank', $back);
});