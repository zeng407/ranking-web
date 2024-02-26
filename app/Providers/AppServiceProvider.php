<?php

namespace App\Providers;

use Illuminate\Routing\Events\RouteMatched;
use Illuminate\Support\ServiceProvider;
use App\Models\Element;
use App\Models\ImgurAlbum;
use App\Models\ImgurImage;
use App\Models\Post;
use App\Models\User;
use Illuminate\Database\Eloquent\Relations\Relation;
use Route;

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
        Relation::enforceMorphMap([
            'post' => Post::class,
            'element' => Element::class,
            'user' => User::class,
            'imgur_image' => ImgurImage::class,
            'imgur_album' => ImgurAlbum::class,
        ]);
        
        if (config('app.force_https')) {
            \URL::forceScheme('https');
        }
    }
}
