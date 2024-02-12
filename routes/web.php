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


// Route::get('/test', function () {
//     return view('sample');
// });

Auth::routes();

Route::get('/lang/{locale}', function($locale){
    Session::put('locale', $locale);
    return redirect()->home();
})->name('lang');


Route::get('/', [HomeController::class, 'index'])->name('home');
Route::get('/hot', [HomeController::class, 'hot'])->name('home.hot');
Route::get('/new', [HomeController::class, 'new'])->name('home.new');

Route::get('account/post', [PostController::class, 'index'])->name('post.index');
Route::get('account/post/{post:serial}/edit', [PostController::class, 'edit'])->name('post.edit');

Route::get('post/{post:serial}/game', [GameController::class, 'show'])->name('game.show');
Route::get('post/{post:serial}/rank', [GameController::class, 'rank'])->name('game.rank');




