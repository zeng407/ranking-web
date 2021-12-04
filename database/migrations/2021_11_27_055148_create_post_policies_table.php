<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;
use App\Enums\PostAccessPolicy;

class CreatePostPoliciesTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('post_policies', function (Blueprint $table) {
            $table->id();
            $table->foreignId('post_id')->constrained();
            $table->enum('access_policy', [
                PostAccessPolicy::PRIVATE,
                PostAccessPolicy::PUBLIC,
                PostAccessPolicy::PASSWORD
            ])->default(PostAccessPolicy::PRIVATE);
            $table->string('password')->nullable();
            $table->timestamps();
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::dropIfExists('post_policies');
    }
}
