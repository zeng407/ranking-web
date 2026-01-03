<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('game_elements', function (Blueprint $table) {
            // Speeds up per-vote updates that filter by game_id and element_id
            $table->index(['game_id', 'element_id'], 'ge_game_element');
            // Speeds up resets that scan by game_id + not eliminated
            $table->index(['game_id', 'is_eliminated'], 'ge_game_elim');
        });

        Schema::table('game_1v1_rounds', function (Blueprint $table) {
            // Speeds up lookups for last round and batches of a game
            $table->index(['game_id', 'current_round', 'of_round'], 'g1r_game_round');
        });
    }

    public function down(): void
    {
        Schema::table('game_elements', function (Blueprint $table) {
            $table->dropIndex('ge_game_element');
            $table->dropIndex('ge_game_elim');
        });

        Schema::table('game_1v1_rounds', function (Blueprint $table) {
            $table->dropIndex('g1r_game_round');
        });
    }
};
