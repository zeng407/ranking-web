<?php

use App\Http\Controllers\Admin\HomeCarouselController;
use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Admin\AdminController;
use App\Http\Controllers\Admin\UserController;
use App\Http\Controllers\Admin\PostController;
use App\Http\Controllers\Admin\AnnouncementController;

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

Route::get('/', [AdminController::class, 'dashboard'])->name('admin.dashboard');
Route::get('/posts', [PostController::class, 'indexPost'])->name('admin.post.index');
Route::get('/post/{post_id}', [PostController::class, 'showPost'])->name('admin.post.show');
Route::put('/post/{post_id}', [PostController::class, 'updatePost'])->name('admin.post.update');
Route::delete('/post/{post_id}', [PostController::class, 'deletePost'])->name('admin.post.delete');
Route::get('/users', [UserController::class, 'index'])->name('admin.user.index');
Route::get('/users/search', [UserController::class, 'search'])->name('admin.user.search');
Route::get('/home-carousel-items', [HomeCarouselController::class, 'index'])->name('admin.home-carousel');
Route::get('/announcements', [AnnouncementController::class, 'index'])->name('admin.announcement.index');
Route::post('/announcement', [AnnouncementController::class, 'create'])->name('admin.announcement.create');
