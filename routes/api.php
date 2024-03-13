<?php

use App\Http\Controllers\Api\TagController;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Api\MyPostController;
use App\Http\Controllers\Api\ElementController;
use App\Http\Controllers\Api\GameController;
use App\Http\Controllers\Api\PublicPostController;
use App\Http\Controllers\Api\RankController;

/** Home */
Route::get('/', [PublicPostController::class, 'index'])->name('api.public-post.index');

/** Tag */
Route::get('tags', [TagController::class, 'index'])->name('api.tag.index');

/** Post */
Route::get('post/{post:serial}/game', [GameController::class, 'getSetting'])->name('api.game.setting');
Route::get('game/{game:serial}/next-round', [GameController::class, 'nextRound'])->name('api.game.next-round');

Route::post('game', [GameController::class, 'create'])->name('api.game.create');
Route::post('game/vote', [GameController::class, 'vote'])->name('api.game.vote');

/** Comment */
Route::get('post/{post:serial}/comments', [PublicPostController::class, 'getComments'])->name('api.public-post.comment.index');
Route::put('post/{post:serial}/comments', [PublicPostController::class, 'createComment'])->name('api.public-post.comment.put');

/** Auth */
Route::middleware(['auth'])->group(function () {
    /** Edit Post */
    Route::get('account/posts', [MyPostController::class, 'index'])->name('api.post.index');
    Route::get('account/post/{post:serial}', [MyPostController::class, 'show'])->name('api.post.show');
    Route::post('account/post', [MyPostController::class, 'create'])->name('api.post.create');
    Route::put('account/post/{post:serial}', [MyPostController::class, 'update'])->name('api.post.update');
    Route::delete('account/post/{post:serial}', [MyPostController::class, 'delete'])->name('api.post.delete');
    Route::get('account/post/{post:serial}/elements', [MyPostController::class, 'elements'])->name('api.post.elements');
    Route::get('account/post/{post:serial}/rank', [MyPostController::class, 'rank'])->name('api.post.rank');

    /** Edit Element */
    Route::post('account/elements/image', [ElementController::class, 'createImage'])->name('api.element.create-image');
    Route::post('account/elements/batch', [ElementController::class, 'batchCreate'])->name('api.element.batch-create');
    Route::put('account/element/{element}', [ElementController::class, 'update'])->name('api.element.update');
    Route::delete('account/element/{element}', [ElementController::class, 'delete'])->name('api.element.delete');
});