<?php

namespace App\Rules;

use Illuminate\Contracts\Validation\Rule;
use App\Models\Post;

class PostPasswordRequired implements Rule
{
    protected Post $post;
    /**
     * Create a new rule instance.
     *
     * @return void
     */
    public function __construct(Post $post)
    {
        $this->post = $post;
    }
    
    /**
     * Determine if the validation rule passes.
     *
     * @param  string  $attribute
     * @param  mixed  $value
     * @return bool
     */
    public function passes($attribute, $value)
    {
        logger($attribute);
        // require new password only when the password is not set
        if($this->post->isPasswordRequired() && $this->post->post_policy->password == null){
            logger('password required');
            if($value == null || empty($value)){
                return false;
            }
            return is_string($value) && strlen($value) >= 0 && strlen($value) <= 255;
        }

        return true;
    }

    /**
     * Get the validation error message.
     *
     * @return string
     */
    public function message()
    {
        return __('Password is required.');
    }
}
