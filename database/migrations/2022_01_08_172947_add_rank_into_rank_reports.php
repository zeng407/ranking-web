<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddRankIntoRankReports extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->unsignedInteger('rank')->nullable()->after('element_id');
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->dropColumn('rank');
        });
    }
}
