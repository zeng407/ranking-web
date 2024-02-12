<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateImgurImagesTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('imgur_images', function (Blueprint $table) {
            $table->id();
            $table->morphs('imageable');
            $table->string('image_id')->nullable();
            $table->foreignId('imgur_album_id')->nullable()->constrained('imgur_albums');
            $table->string('title')->nullable();
            $table->string('description')->nullable();
            $table->string('delete_hash')->nullable();
            $table->string('link')->nullable();
            $table->softDeletes();
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
        Schema::dropIfExists('imgur_images');
    }
}
