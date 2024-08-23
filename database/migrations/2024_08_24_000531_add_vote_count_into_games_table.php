<?php

use App\Models\Game;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddVoteCountIntoGamesTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('games', function (Blueprint $table) {
            $table->integer('vote_count')->default(0)->after('element_count');
        });

        Game::withCount('game_1v1_rounds')
            ->eachById(function (Game $game) {
                $game->update(['vote_count' => $game->game_1v1_rounds_count]);
            });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        if(app()->environment() === 'testing') {
            return;
        }
        Schema::table('games', function (Blueprint $table) {
            $table->dropColumn('vote_count');
        });
    }
}
