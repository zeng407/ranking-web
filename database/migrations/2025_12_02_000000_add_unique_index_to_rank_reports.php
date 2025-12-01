<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        $this->removeActiveDuplicates();

        Schema::table('rank_reports', function (Blueprint $table) {
            $table->unique(['post_id', 'element_id', 'deleted_at'], 'rank_reports_post_element_unique');
        });
    }

    public function down(): void
    {
        Schema::table('rank_reports', function (Blueprint $table) {
            $table->dropUnique('rank_reports_post_element_unique');
        });
    }

    private function removeActiveDuplicates(): void
    {
        $duplicates = DB::table('rank_reports')
            ->select('post_id', 'element_id', DB::raw('deleted_at as deleted_at_value'), DB::raw('MIN(id) as keep_id'))
            ->groupBy('post_id', 'element_id', 'deleted_at_value')
            ->havingRaw('COUNT(*) > 1')
            ->get();

        foreach ($duplicates as $duplicate) {
            DB::table('rank_reports')
                ->where('post_id', $duplicate->post_id)
                ->where('element_id', $duplicate->element_id)
                ->where(function ($query) use ($duplicate) {
                    if (is_null($duplicate->deleted_at_value)) {
                        $query->whereNull('deleted_at');
                    } else {
                        $query->where('deleted_at', $duplicate->deleted_at_value);
                    }
                })
                ->where('id', '!=', $duplicate->keep_id)
                ->delete();
        }
    }
};
