<?php

use App\Http\Controllers\Api\HomeCarouselController;
use App\Http\Controllers\Api\PublicPostController;
use App\Http\Controllers\Api\RankController;
use App\Http\Controllers\Api\TagController;
use Illuminate\Support\Facades\Route;

/** Tag */
Route::get('tags', [TagController::class, 'index'])->name('api.tag.index');
Route::get('tags/hot', [TagController::class, 'hot'])->name('api.tag.hot');

/** Carousel */
Route::get('carousel-items', [HomeCarouselController::class, 'index'])->name('api.carousel.index');

/** Post */
Route::get('posts', [PublicPostController::class, 'getPosts'])->name('api.public-post.index');
Route::get('champions', [PublicPostController::class, 'getChampions'])->name('api.champion.index');

/** Rank */
Route::get('rank', [RankController::class, 'getPublicRank'])->name('api.rank.index');
Route::get('rank/search', [RankController::class, 'searchPublicRank'])->name('api.rank.search');
