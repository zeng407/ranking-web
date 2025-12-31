<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddUniqueIndexToRankReports extends Migration
{
    public function up()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->dropUnique('rank_reports_post_element_unique');
            $table->dropSoftDeletes();
        });

        DB::statement("
            DELETE t1 FROM rank_reports t1
            INNER JOIN rank_reports t2
            WHERE
                t1.post_id = t2.post_id AND
                t1.element_id = t2.element_id AND
                t1.id <> t2.id AND
                (
                    -- 情況 A: t1 是隱藏的，但 t2 是顯示的 -> 刪除 t1 (不管 ID 誰大誰小)
                    (t1.hidden = 1 AND t2.hidden = 0)
                    OR
                    -- 情況 B: 兩者狀態相同 (都是顯示 或 都是隱藏) -> 刪除 ID 較小的 t1 (保留較新的)
                    (t1.hidden = t2.hidden AND t1.id < t2.id)
                )
        ");

        Schema::table('rank_reports', function (Blueprint $table) {
            $table->unique(['post_id', 'element_id'], 'unique_post_element');
        });
    }

    public function down()
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->softDeletes();
            $table->dropUnique('unique_post_element');
            $table->unique(['post_id', 'element_id', 'deleted_at'], 'rank_reports_post_element_unique');
        });
    }
};
