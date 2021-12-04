<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateGameRoundsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('game_1v1_rounds', function (Blueprint $table) {
            $table->id();
            $table->foreignId('game_id')->constrained();
            $table->unsignedInteger('current_round');
            $table->unsignedInteger('of_round');
            $table->unsignedInteger('remain_elements');
            $table->unsignedBigInteger('winner_id')->nullable();
            $table->unsignedBigInteger('loser_id')->nullable();
            $table->timestamp('complete_at')->nullable();

            $table->foreign('winner_id')->references('id')->on('elements');
            $table->foreign('loser_id')->references('id')->on('elements');
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
        Schema::dropIfExists('game_1v1_rounds');
    }
}
