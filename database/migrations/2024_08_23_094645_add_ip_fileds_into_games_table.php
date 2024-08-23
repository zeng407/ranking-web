<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddIpFiledsIntoGamesTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('games', function (Blueprint $table) {
            $table->string('ip')->nullable()->after('completed_at');
            $table->string('ip_country')->nullable()->after('ip');
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
        Schema::table('games', function (Blueprint $table) {
            $table->dropColumn('ip');
            $table->dropColumn('ip_country');
        });
    }
}
