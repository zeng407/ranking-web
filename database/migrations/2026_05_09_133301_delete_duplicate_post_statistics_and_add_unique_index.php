<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class DeleteDuplicatePostStatisticsAndAddUniqueIndex extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        // 1. 刪除 time_range 為 NULL 的資料
        DB::table('post_statistics')->whereNull('time_range')->delete();

        // 2. 將 time_range 欄位改為 NOT NULL
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->string('time_range')->nullable(false)->change();
        });

        // 3. 找出重複的資料並保留第一筆
        if(app()->environment('production')) {
            DB::statement("
            DELETE t1 FROM post_statistics t1
            INNER JOIN post_statistics t2
            WHERE
                t1.id > t2.id AND
                t1.post_id = t2.post_id AND
                t1.start_date = t2.start_date AND
                t1.time_range = t2.time_range
            ");
        }

        // 4. 加入 Unique Index
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->unique(['post_id', 'start_date', 'time_range'], 'post_statistics_unique_index');
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::table('post_statistics', function (Blueprint $table) {
            $table->dropUnique('post_statistics_unique_index');
        });
    }
}
