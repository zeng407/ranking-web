<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddLoserIdIntoUserGameResults extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('user_game_results', function (Blueprint $table) {
            $table->foreignId('loser_id')->nullable()->after('champion_id')->constrained('elements')->onDelete('set null');
            $table->string('loser_name')->nullable()->after('loser_id');
            $table->string('candidates')->nullable()->after('champion_name');
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
        Schema::table('user_game_results', function (Blueprint $table) {
            $table->dropForeign(['loser_id']);
            $table->dropColumn(['loser_id', 'loser_name', 'candidates']);
        });
    }
}
