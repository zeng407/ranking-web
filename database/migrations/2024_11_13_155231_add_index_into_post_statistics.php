<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddIndexIntoPostStatistics extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->index('play_count');
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        if(app()->environment() === 'testing') {
            return;
        }
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->dropIndex(['play_count']);
        });
    }
}
