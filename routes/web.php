<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\HomeController;
use App\Http\Controllers\Post\PostController;

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

Route::get('post/create', [PostController::class, 'create'])->name('post.create');
Route::get('post/{serial}', [PostController::class, 'show'])->name('post.show');
Route::get('rank/{serial}', [PostController::class, 'rank'])->name('post.rank');




