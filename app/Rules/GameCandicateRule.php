<?php

namespace App\Rules;

use App\Models\Game;
use Illuminate\Contracts\Validation\Rule;

class GameCandicateRule implements Rule
{
    protected Game $game;

    /**
     * Create a new rule instance.
     *
     * @return void
     */
    public function __construct(Game $game)
    {
        $this->game = $game;
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
        $cadiates = explode(',', $this->game->candidates?:'');
        return in_array($value, $cadiates);
    }

    /**
     * Get the validation error message.
     *
     * @return string
     */
    public function message()
    {
        return 'Invalid candidate.';
    }
}
