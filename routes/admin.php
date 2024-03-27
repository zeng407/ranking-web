<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\Admin\AdminController;
use App\Http\Controllers\Admin\UserController;

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
Route::get('/posts', [AdminController::class, 'indexPost'])->name('admin.post.index');
Route::get('/post/{post_id}', [AdminController::class, 'showPost'])->name('admin.post.show');
Route::put('/post/{post_id}', [AdminController::class, 'updatePost'])->name('admin.post.update');
Route::delete('/post/{post_id}', [AdminController::class, 'deletePost'])->name('admin.post.delete');
Route::get('/users', [UserController::class, 'index'])->name('admin.user.index');
Route::get('/users/search', [UserController::class, 'search'])->name('admin.user.search');