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
        User::factory(1)->has(
            Post::factory(11)->has(
                PostPolicy::factory()->public(),
                'post_policy'
            )
        )->create();

        User::each(function (User $user) {
            $elements = [256,128,65,63,32,33,8,7,1,9,0,0];
            $user->posts()->each(function (Post $post) use( &$elements) {
                Element::factory(array_shift($elements))->hasAttached($post)->create();
            });
        });


    }
}
