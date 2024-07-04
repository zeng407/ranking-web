<?php

use App\Http\Controllers\Api\HomeCarouselController;
use App\Http\Controllers\Api\RankController;
use App\Http\Controllers\Api\TagController;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Api\MyPostController;
use App\Http\Controllers\Api\ElementController;
use App\Http\Controllers\Api\GameController;
use App\Http\Controllers\Api\PublicPostController;

/** Tag */
Route::get('tags', [TagController::class, 'index'])->name('api.tag.index');

/** Carousel */
Route::get('carousel-items', [HomeCarouselController::class, 'index'])->name('api.carousel.index');

/** Post */
Route::get('posts', [PublicPostController::class, 'getPosts'])->name('api.public-post.index');
Route::get('champions', [PublicPostController::class, 'getChampions'])->name('api.champion.index');

/** Rank */
Route::get('rank', [RankController::class, 'getRank'])->name('api.rank.index');

/** Game */
Route::get('post/{post:serial}/game', [GameController::class, 'getSetting'])->name('api.game.setting');
Route::get('post/{post:serial}/access', [GameController::class, 'access'])->name('api.game.access');
Route::get('game/{game:serial}/next-round', [GameController::class, 'nextRound'])->name('api.game.next-round');

Route::post('game', [GameController::class, 'create'])->name('api.game.create');
Route::post('game/vote', [GameController::class, 'vote'])->name('api.game.vote');

/** Comment */
Route::get('post/{post:serial}/comments', [PublicPostController::class, 'getComments'])->name('api.public-post.comment.index');
Route::post('post/{post:serial}/comments', [PublicPostController::class, 'createComment'])->name('api.public-post.comment.create');
Route::post('post/{post:serial}/comment/{comment:id}/report', [PublicPostController::class, 'report'])->name('api.public-post.comment.report');

/** Auth */
Route::middleware(['auth'])->group(function () {
    /** Edit Post */
    Route::get('account/posts', [MyPostController::class, 'index'])->name('api.post.index');
    Route::get('account/post/{post:serial}', [MyPostController::class, 'show'])->name('api.post.show');
    Route::post('account/post', [MyPostController::class, 'create'])->name('api.post.create');
    Route::put('account/post/{post:serial}', [MyPostController::class, 'update'])->name('api.post.update');
    Route::delete('account/post/{post:serial}', [MyPostController::class, 'delete'])->name('api.post.delete');
    Route::get('account/post/{post:serial}/elements', [MyPostController::class, 'elements'])->name('api.post.elements');

    /** Edit Element */
    Route::post('account/elements/media', [ElementController::class, 'createMedia'])->name('api.element.create-media');
    Route::post('account/elements/batch', [ElementController::class, 'batchCreate'])->name('api.element.batch-create');
    Route::put('account/element/{element}', [ElementController::class, 'update'])->name('api.element.update');
    Route::post('account/element/{element}/upload', [ElementController::class, 'upload'])->name('api.element.upload');
    Route::delete('account/element/{element}', [ElementController::class, 'delete'])->name('api.element.delete');
});

/** Game Room */
Route::post('game/room', [GameController::class, 'createRoom'])->name('api.game-room.create');
Route::get('game/room/{gameRoom:serial}', [GameController::class, 'getRoom'])->name('api.game-room.get');
Route::get('game/room/{gameRoom:serial}/votes', [GameController::class, 'getRoomVotes'])->name('api.game-room.get-votes');
Route::get('game/room/{gameRoom:serial}/user', [GameController::class, 'getRoomUser'])->name('api.game-room.get-user');
Route::post('game/room/{gameRoom:serial}/bet', [GameController::class, 'bet'])->name('api.game-room.bet');
Route::put('game/room/{gameRoom:serial}/profile', [GameController::class, 'updateGameUser'])->name('api.game-room.update-profile');

