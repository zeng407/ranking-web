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

Route::get('post/{post:serial}/game', [GameController::class, 'getSetting'])->name('api.game.setting');
Route::get('game/{game:serial}/next-round', [GameController::class, 'nextRound'])->name('api.game.next-round');
Route::get('game/{game:serial}/result', [GameController::class, 'result'])->name('api.game.result');
Route::post('game', [GameController::class, 'create'])->name('api.game.create');
Route::post('game/vote', [GameController::class, 'vote'])->name('api.game.vote');
//Route::get('post/{post:serial}/game/{game:serial}', [GameController::class, 'nextRound'])->name('api.game.next-round');



Route::get('post/{post:serial}/rank', [RankController::class, 'index'])->name('api.rank.index');
Route::get('post/{post:serial}/rank/report', [RankController::class, 'report'])->name('api.rank.report');


Route::get('posts', [PostController::class, 'index'])->name('api.post.index');
Route::get('post/{post:serial}', [PostController::class, 'show'])->name('api.post.show');
Route::post('post', [PostController::class, 'create'])->name('api.post.create');
Route::put('post/{post:serial}', [PostController::class, 'update'])->name('api.post.update');
Route::get('post/{post:serial}/elements', [PostController::class, 'elements'])->name('api.post.elements');
Route::post('elements/image', [ElementController::class, 'createImage'])->name('api.element.create-image');
Route::post('elements/video', [ElementController::class, 'createVideo'])->name('api.element.create-video');
Route::put('element/{element}', [ElementController::class, 'update'])->name('api.element.update');
Route::delete('element/{element}', [ElementController::class, 'delete'])->name('api.element.delete');


