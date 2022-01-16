<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreatePostTrends extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('post_trends', function (Blueprint $table) {
            $table->id();
            $table->foreignId('post_id')->constrained('posts');
            $table->string('trend_type', '50')->index();
            $table->string('time_range', '50')->index();
            $table->date('start_date')->index()->nullable();
            $table->unsignedInteger('position')->index();
            $table->timestamps();
        });
    }

    /**
     * Reverse the migrations.
     **
     * @return void
     */
    public function down()
    {
        Schema::dropIfExists('post_trends');
    }
}
