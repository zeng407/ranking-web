<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddIndexesToGamesAndRounds extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        DB::statement('CREATE INDEX idx_games_post_completed_id ON games (post_id, completed_at, id)');
        DB::statement('CREATE INDEX idx_rounds_game_winner ON game_1v1_rounds (game_id, winner_id)');
        DB::statement('CREATE INDEX idx_rounds_game_loser ON game_1v1_rounds (game_id, loser_id)');
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::table('games', function (Blueprint $table) {
            $table->dropIndex('idx_games_post_completed_id');
        });
        Schema::table('game_1v1_rounds', function (Blueprint $table) {
            $table->dropIndex('idx_rounds_game_winner');
            $table->dropIndex('idx_rounds_game_loser');
        });
    }
}
