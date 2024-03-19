<?php

use App\Http\Controllers\Profile\ProfileController;
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

Route::get('/lang/{locale}', function($locale){
    Session::put('locale', $locale);
    return redirect()->home();
})->name('lang');

Route::get('/', [HomeController::class, 'index'])->name('home');
Route::get('post/{post:serial}/game', [GameController::class, 'show'])->name('game.show');
Route::get('post/{post:serial}/rank', [GameController::class, 'rank'])->name('game.rank');

Route::middleware('auth')->group(function () {
    Route::get('account/profile', [ProfileController::class, 'index'])->name('profile.index');
    Route::put('account/profile', [ProfileController::class, 'update'])->name('profile.update');
    Route::put('account/password', [ProfileController::class, 'updatePassword'])->name('profile.update.password');
    Route::get('account/post', [PostController::class, 'index'])->name('post.index');
    Route::get('account/post/{post:serial}/edit', [PostController::class, 'edit'])->name('post.edit');
});




