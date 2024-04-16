<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class DropCompleteAtOnGame1v1RoundsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('game_1v1_rounds', function (Blueprint $table) {
            $table->dropColumn('complete_at');
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::table('game_1v1_rounds', function (Blueprint $table) {
            $table->timestamp('complete_at')->nullable()->after('loser_id');
        });
    }
}
