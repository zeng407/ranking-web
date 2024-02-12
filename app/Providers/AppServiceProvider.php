<?php

namespace App\Providers;

use App\Models\Element;
use App\Models\ImgurAlbum;
use App\Models\ImgurImage;
use App\Models\Post;
use App\Models\User;
use Illuminate\Support\ServiceProvider;
use Illuminate\Database\Eloquent\Relations\Relation;

class AppServiceProvider extends ServiceProvider
{
    /**
     * Register any application services.
     *
     * @return void
     */
    public function register()
    {
        if ($this->app->isLocal()) {
            $this->app->register(\Barryvdh\LaravelIdeHelper\IdeHelperServiceProvider::class);
        }
    }

    /**
     * Bootstrap any application services.
     *
     * @return void
     */
    public function boot()
    {
        /**
         * Enforce the morph map for the polymorphic relationships.
         */
        Relation::enforceMorphMap([
            'post' => Post::class,
            'element' => Element::class,
            'user' => User::class,
            'imgur_image' => ImgurImage::class,
            'imgur_album' => ImgurAlbum::class,
        ]);
    }
}
