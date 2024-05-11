<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Admin\Api\UserController;
use App\Http\Controllers\Admin\Api\ElementController;
use App\Http\Controllers\Admin\Api\HomeCarouselController;

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


Route::get('post/{post_id}/elements', [ElementController::class, 'indexElement'])->name('admin.api.element.index');
Route::put('post/{post_id}/element/{element_id}', [ElementController::class, 'updateElement'])->name('admin.api.element.update');
Route::delete('post/{post_id}/element/{element_id}', [ElementController::class, 'deleteElement'])->name('admin.api.element.delete');
Route::put('user/{user_id}/ban', [UserController::class, 'ban'])->name('admin.user.ban');
Route::put('user/{user_id}/unban', [UserController::class, 'unban'])->name('admin.user.unban');

Route::get('home-carousel-items', [HomeCarouselController::class, 'index'])->name('admin.api.home-carousel.index');
Route::post('home-carousel-items', [HomeCarouselController::class, 'create'])->name('admin.api.home-carousel.create');
Route::delete('home-carousel-item/{item_id}', [HomeCarouselController::class, 'delete'])->name('admin.api.home-carousel.delete');
Route::put('home-carousel-items/{item_id}', [HomeCarouselController::class, 'update'])->name('admin.api.home-carousel.update');
Route::put('home-carousel-item/reorder', [HomeCarouselController::class, 'reorder'])->name('admin.api.home-carousel.reorder');