<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;
use App\Enums\ElementType;

class CreateElementsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('elements', function (Blueprint $table) {
            $table->id();
            $table->string('path')->nullable();
            $table->string('source_url')->nullable();
            $table->string('thumb_url')->nullable();
            $table->string('title')->nullable();
            $table->enum('type', [
                ElementType::IMAGE,
                ElementType::VIDEO
            ]);
            $table->string('video_source')->nullable();
            $table->string('video_id')->nullable();
            $table->unsignedInteger('video_duration_second')->nullable();
            $table->unsignedInteger('video_start_second')->nullable();
            $table->unsignedInteger('video_end_second')->nullable();
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
        Schema::dropIfExists('elements');
    }
}
