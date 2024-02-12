<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateImgurAlbumsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('imgur_albums', function (Blueprint $table) {
            $table->id();
            $table->morphs('albumable');
            $table->string('album_id')->nullable();
            $table->string('title');
            $table->string('description');
            $table->string('delete_hash')->nullable();
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
        Schema::dropIfExists('imgur_albums');
    }
}