<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddIsReadyIntoGameElements extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('game_elements', function (Blueprint $table) {
            $table->boolean('is_ready')->default(false)->after('is_eliminated');
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
        Schema::table('game_elements', function (Blueprint $table) {
            $table->dropColumn('is_ready');
        });
    }
}
