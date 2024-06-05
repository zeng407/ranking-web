<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddIndexIntoRankReportHistoryies extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('rank_report_histories', function (Blueprint $table) {
            $table->index(['start_date']);
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
        Schema::table('rank_report_histories', function (Blueprint $table) {
            $table->dropIndex(['start_date']);
        });
    }
}
