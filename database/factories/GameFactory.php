<?php

namespace Database\Factories;

use App\Enums\ElementType;
use Illuminate\Database\Eloquent\Factories\Factory;
use Illuminate\Support\Str;

class GameFactory extends Factory
{
    /**
     * Define the model's default state.
     *
     * @return array
     */
    public function definition()
    {
        return [
            'serial' => Str::random(8),
            'element_count' => random_int(config('setting.post_min_element_count'), config('setting.post_max_element_count'))
        ];
    }


}
