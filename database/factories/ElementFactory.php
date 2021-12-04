<?php

namespace Database\Factories;

use App\Enums\ElementType;
use App\Models\Element;
use Closure;
use Illuminate\Database\Eloquent\Factories\Factory;
use Illuminate\Support\Str;

class ElementFactory extends Factory
{
    /**
     * Define the model's default state.
     *
     * @return array
     */
    public function definition()
    {
        return [
            'path' => $this->faker->filePath(),
            'source_url' => $this->faker->imageUrl,
            'thumb_url' => $this->faker->imageUrl,
            'title' => $this->faker->word,
            'type' => $this->faker->randomElement([ElementType::IMAGE, ElementType::VIDEO]),
            'video_start_second' => null,
            'video_end_second' => null
        ];
    }

    public function image()
    {
        return $this->state(function (array $attributes) {
            return [
                'type' => ElementType::IMAGE,
                'video_start_second' => null,
                'video_end_second' => null
            ];
        });
    }

    public function video()
    {
        return $this->state(function (array $attributes) {
            return [
                'type' => ElementType::VIDEO,
                'video_start_second' => rand(0,20),
                'video_end_second' => rand(21,50)
            ];
        });
    }

}
