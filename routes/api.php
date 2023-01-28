<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Api\MyPostController;
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


/**
 * Auth
 */
Route::get('account/posts', [MyPostController::class, 'index'])->name('api.post.index');
Route::get('account/post/{post:serial}', [MyPostController::class, 'show'])->name('api.post.show');
Route::post('account/post', [MyPostController::class, 'create'])->name('api.post.create');
Route::put('account/post/{post:serial}', [MyPostController::class, 'update'])->name('api.post.update');
Route::get('account/post/{post:serial}/elements', [MyPostController::class, 'elements'])->name('api.post.elements');
Route::get('account/post/{post:serial}/rank', [MyPostController::class, 'rank'])->name('api.post.rank');
Route::post('account/elements/image', [ElementController::class, 'createImage'])->name('api.element.create-image');
Route::post('account/elements/image-url', [ElementController::class, 'createImageUrl'])->name('api.element.create-image-url');
Route::post('account/elements/video-youtube', [ElementController::class, 'createVideoYoutube'])->name('api.element.create-video-youtube');
Route::post('account/elements/video-url', [ElementController::class, 'createVideoUrl'])->name('api.element.create-video-url');
Route::put('account/element/{element}', [ElementController::class, 'update'])->name('api.element.update');
Route::delete('account/element/{element}', [ElementController::class, 'delete'])->name('api.element.delete');


