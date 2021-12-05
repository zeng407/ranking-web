<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Api\PostController;
use App\Http\Controllers\Api\ElementController;
use App\Http\Controllers\Api\GameController;
use App\Http\Controllers\Api\PublicPostController;
use App\Http\Controllers\Api\RankController;

/*
|--------------------------------------------------------------------------
| API Routes
|--------------------------------------------------------------------------
|
| Here is where you can register API routes for your application. These
| routes are loaded by the RouteServiceProvider within a group which
| is assigned the "api" middleware group. Enjoy building your API!
|
*/

// Route::middleware('auth:sanctum')->get('/user', function (Request $request) {
//     return $request->user();
// });

Route::get('/', [PublicPostController::class, 'index'])->name('api.public-post.index');

Route::get('game/{serial}', [GameController::class, 'show'])->name('api.game.show');
Route::post('game', [GameController::class, 'create'])->name('api.game.create');
Route::post('game/vote', [GameController::class, 'vote'])->name('api.game.vote');

Route::get('rank/{serial}', [RankController::class, 'show'])->name('api.rank.show');


Route::get('posts', [PostController::class, 'index'])->name('api.post.index');
Route::post('post', [PostController::class, 'create'])->name('api.post.create');
Route::put('post/{serial}', [PostController::class, 'update'])->name('api.post.update');
Route::post('elements/image', [ElementController::class, 'createImage'])->name('api.element.create-image');
Route::post('elements/video', [ElementController::class, 'createVideo'])->name('api.element.create-video');


