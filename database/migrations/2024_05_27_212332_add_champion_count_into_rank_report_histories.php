<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddChampionCountIntoRankReportHistories extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('rank_report_histories', function (Blueprint $table) {
            $table->unsignedInteger('champion_count')->default(0)->after('win_rate');
            $table->unsignedInteger('game_complete_count')->default(0)->after('champion_count');
            $table->decimal('champion_rate')->default(0)->after('game_complete_count');
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::table('rank_report_histories', function (Blueprint $table) {
            $table->dropColumn('champion_count');
            $table->dropColumn('game_complete_count');
            $table->dropColumn('champion_rate');
        });
    }
}
