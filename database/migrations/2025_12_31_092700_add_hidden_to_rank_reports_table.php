<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddHiddenToRankReportsTable extends Migration
{
    public function up()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->boolean('hidden')->default(false)->index();
        });

        DB::table('rank_reports')
            ->whereNotNull('deleted_at')
            ->update(['hidden' => true]);
    }

    public function down()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->dropColumn('hidden');
        });
    }
}
