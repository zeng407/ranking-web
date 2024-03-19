<?php

namespace Database\Factories;

use App\Models\Comment;
use Illuminate\Database\Eloquent\Factories\Factory;
use Ramsey\Uuid\Uuid;

class CommentFactory extends Factory
{
    protected $model = Comment::class;

    public function definition()
    {
        return [
            'content' => $this->faker->text(100),
            'user_id' => function () {
                return \App\Models\User::factory()->create()->id;
            },
            'delete_hash' => Uuid::uuid4()->toString(),
            'anonymous_id' => $this->faker->uuid,
            'nickname' => $this->faker->name,
            'ip' => $this->faker->ipv4,
        ];
    }
}
