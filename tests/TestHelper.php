<?php

namespace Tests;

use App\Models\Post;
use App\Models\PostPolicy;
use App\Models\User;
use App\Models\Element;


trait TestHelper
{
    public function createPost(): Post
    {
        return Post::factory()->has(
            PostPolicy::factory()->public(),
            'post_policy'
        )->for(User::factory()->make())->create();
    }

    public function createElements(Post $post, $number = 1)
    {
        return Element::factory($number)->hasAttached($post)->create();
    }
}