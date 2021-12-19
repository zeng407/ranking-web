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
Route::get('/', [HomeController::class, 'index'])->name('home');

Route::get('post', [PostController::class, 'index'])->name('post.index');
Route::get('post/{serial}', [PostController::class, 'edit'])->name('post.edit');

Route::get('game/{serial}', [GameController::class, 'show'])->name('game.show');
Route::get('game/{serial}/rank', [GameController::class, 'rank'])->name('game.rank');




