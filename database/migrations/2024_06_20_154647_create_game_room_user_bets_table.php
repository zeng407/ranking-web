<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateGameRoomUserBetsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('game_room_user_bets', function (Blueprint $table) {
            $table->id();
            $table->foreignId('game_room_id')->constrained()->cascadeOnDelete();
            $table->foreignId('game_room_user_id')->constrained()->cascadeOnDelete();
            $table->unsignedInteger('current_round');
            $table->unsignedInteger('of_round');
            $table->unsignedInteger('remain_elements');
            $table->foreignId('winner_id')->constrained('elements')->cascadeOnDelete();
            $table->foreignId('loser_id')->constrained('elements')->cascadeOnDelete();
            $table->timestamp('won_at')->nullable();
            $table->timestamp('lost_at')->nullable();
            $table->unsignedSmallInteger('last_combo')->default(0);
            $table->smallInteger('score')->default(0);

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
        Schema::dropIfExists('game_room_user_bets');
    }
}
