<?php

namespace App\Providers;

use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Policies\ElementPolicy;
use App\Policies\GamePolicy;
use App\Policies\PostPolicy;
use Illuminate\Foundation\Support\Providers\AuthServiceProvider as ServiceProvider;
use Illuminate\Support\Facades\Gate;

class AuthServiceProvider extends ServiceProvider
{
    /**
     * The policy mappings for the application.
     *
     * @var array
     */
    protected $policies = [
        Element::class => ElementPolicy::class,
        Post::class => PostPolicy::class,
        Game::class => GamePolicy::class
    ];

    /**
     * Register any authentication / authorization services.
     *
     * @return void
     */
    public function boot()
    {
        $this->registerPolicies();

        //
    }
}
