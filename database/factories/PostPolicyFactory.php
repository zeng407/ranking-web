<?php

namespace Database\Factories;

use App\Enums\ElementType;
use App\Enums\PostAccessPolicy;
use Illuminate\Database\Eloquent\Factories\Factory;
use Illuminate\Support\Str;
use Hash;

class PostPolicyFactory extends Factory
{
    /**
     * Define the model's default state.
     *
     * @return array
     */
    public function definition()
    {
        return [
            'access_policy' => $this->faker->randomElement([
                PostAccessPolicy::PUBLIC,
                PostAccessPolicy::PRIVATE,
                PostAccessPolicy::PASSWORD
            ]),
            'password' => Hash::make(Str::random(8))
        ];
    }

    public function public()
    {
        return $this->state(function (array $attributes) {
            return [
                'access_policy' => PostAccessPolicy::PUBLIC,
                'password' => null
            ];
        });
    }


}
