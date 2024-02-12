<?php

namespace App\Policies;

use App\Models\Element;
use App\Models\Post;
use App\Models\User;
use Illuminate\Auth\Access\HandlesAuthorization;

class PostPolicy
{
    use HandlesAuthorization;

    public function publicRead(?User $user, Post $post)
    {
        return $post->isPublic() || $post->user_id === optional($user)->id;
    }

    public function read(User $user, Post $post)
    {
        return $post->user_id === $user->id;
    }

    public function edit(User $user, Post $post)
    {
        return $post->user_id === $user->id;
    }

    public function update(User $user, Post $post)
    {
        return $post->user_id === $user->id;
    }

    public function create(User $user)
    {
        return true;
    }

    public function delete(User $user, Post $post)
    {
        return $post->user_id === $user->id;
    }

    public function newGame(?User $user, Post $post)
    {
        if($user && $user->id === $post->user_id){
            return true;
        }

        return $post->isPublic();
    }
}

