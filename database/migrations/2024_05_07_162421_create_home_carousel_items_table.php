<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateHomeCarouselItemsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('home_carousel_items', function (Blueprint $table) {
            $table->id();
            $table->unsignedInteger('position');
            $table->string('type');
            $table->string('title')->nullable();
            $table->string('description')->nullable();
            $table->string('image_url')->nullable();
            $table->string('video_url')->nullable();
            $table->string('video_source')->nullable();
            $table->string('video_id')->nullable();
            $table->string('video_start_second')->nullable();
            $table->string('video_end_second')->nullable();

            $table->timestamps();

            $table->softDeletes();
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::dropIfExists('home_carousel_items');
    }
}
