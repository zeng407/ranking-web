<?php

namespace App\Policies;

use App\Helper\AccessTokenService;
use App\Models\Element;
use App\Models\Post;
use App\Models\User;
use Illuminate\Auth\Access\AuthorizationException;
use Illuminate\Auth\Access\HandlesAuthorization;

class PostPolicy
{
    use HandlesAuthorization;

    const INVALID_PASSWORD = 'Invalid password';

    public function publicRead(?User $user, Post $post, ?string $password = null)
    {
        if($post->isPasswordRequired()){
            logger('publicRead', [$password, $post->post_policy->password]);
            if(AccessTokenService::verifyPostAccessToken($post)){
                return true;
            }

            if(empty($password)){
                return false;
            }
            
            if(hash('sha256',$password) !== $post->post_policy->password){
                throw new AuthorizationException(self::INVALID_PASSWORD, 403);
            }
            
            return true;
        }

        if($post->user_id === optional($user)->id){
            return true;
        }

        return $post->isPublic();
    }

    public function publicRank(?User $user, Post $post)
    {
        if($post->user_id === optional($user)->id){
            return true;
        }

        return $post->isPublic();
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

    public function newGame(?User $user, Post $post, ?string $password = null)
    {
        logger('newGame', [$password, $post->post_policy->password]);
        if($post->isPasswordRequired()){
            if(AccessTokenService::verifyPostAccessToken($post)){
                AccessTokenService::extendPostAccessToken($post);
                return true;
            }

            if(empty($password)){
                return false;
            }

            logger('newGame', [$password, $post->post_policy->password]);
            return hash('sha256', $password) === $post->post_policy->password;
        }

        if($user && $user->id === $post->user_id){
            return true;
        }

        return $post->isPublic();
    }
}

