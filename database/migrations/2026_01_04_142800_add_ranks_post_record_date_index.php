<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddRanksPostRecordDateIndex extends Migration
{
    public function up()
    {
        Schema::table('ranks', function (Blueprint $table) {
            $table->index(['post_id', 'record_date'], 'ranks_post_record_date_idx');
        });
    }

    public function down()
    {
        Schema::table('ranks', function (Blueprint $table) {
            $table->dropIndex('ranks_post_record_date_idx');
        });
    }
}
