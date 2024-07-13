<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class AddMediumthumbUrlIntoElements extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('elements', function (Blueprint $table) {
            $table->text('mediumthumb_url')->nullable()->after('thumb_url');
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
        Schema::table('elements', function (Blueprint $table) {
            $table->dropColumn('mediumthumb_url');
        });
    }
}
