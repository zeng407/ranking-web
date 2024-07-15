<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddTimeRangeIntoPostStatistics extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->string('time_range')->after('post_id')->nullable();
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        if(app()->environment() === 'testing'){
            return;
        }
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->dropColumn('time_range');
        });
    }
}
