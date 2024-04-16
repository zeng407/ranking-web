<?php

use App\Models\Game;
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddCompletedAtIntoGames extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('games', function (Blueprint $table) {
            $table->timestamp('completed_at')->nullable()->after('candidates');
        });

        Game::whereNull('completed_at')
            ->whereHas('game_1v1_rounds', function($query){
                $query->where('remain_elements', 1);
            })
            ->with('game_1v1_rounds')
            ->chunkById(100, function ($games) {
            foreach ($games as $game) {
                $round = $game->game_1v1_rounds->firstWhere('remain_elements', 1);
                $game->update([
                    'completed_at' => $round->created_at,
                ]);
                dump($game->id . ' completed at ' . $game->completed_at);
            }
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        if(app()->environment('testing')){
            return;
        }

        Schema::table('games', function (Blueprint $table) {
            $table->dropColumn('completed_at');
        });
    }
}
