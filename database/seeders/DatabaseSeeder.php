<?php

namespace Database\Seeders;

use App\Models\Element;
use App\Models\Post;
use App\Models\PostPolicy;
use Database\Factories\ElementFactory;
use Illuminate\Database\Seeder;
use App\Models\User;

class DatabaseSeeder extends Seeder
{
    /**
     * Seed the application's database.
     *
     * @return void
     */
    public function run()
    {
        User::factory(10)->has(
            Post::factory(rand(2, 5))->has(
                PostPolicy::factory()->public(),
                'post_policy'
            )
        )->create();

        User::each(function (User $user) {
            $user->posts()->each(function (Post $post) {
                Element::factory(rand(4, 32))->hasAttached($post)->create();
                Element::factory(rand(4, 10))->hasAttached($post)->video()->create();
            });
        });


    }
}
