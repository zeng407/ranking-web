<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateGameRoomUsersTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('game_room_users', function (Blueprint $table) {
            $table->id();
            $table->foreignId('game_room_id')->constrained()->onDelete('cascade');
            $table->foreignid('user_id')->nullable()->constrained()->setNullOnDelete();
            $table->string('anonymous_id')->index();
            $table->string('nickname', 20);
            $table->integer('score')->default(0);
            $table->unsignedInteger('rank')->default(0);
            $table->decimal('accuracy', 5, 2)->default(0);
            $table->smallInteger('combo')->default(0);
            $table->unsignedInteger('total_played')->default(0);
            $table->unsignedInteger('total_correct')->default(0);
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
        Schema::dropIfExists('game_room_users');
    }
}
