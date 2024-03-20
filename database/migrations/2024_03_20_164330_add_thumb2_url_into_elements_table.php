<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;
use App\Models\Element;

class AddThumb2UrlIntoElementsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::table('elements', function (Blueprint $table) {
            $table->string('thumb2_url')->nullable()->after('thumb_url');
        });

        Element::where('type', 'image')->chunkById(100, function ($elements) {
            foreach ($elements as $element) {
                try {
                    $element->thumb2_url = Storage::url($element->path);
                    $element->save();
                } catch (\Exception $e) {
                    \Log::error($e->getMessage());
                }
            }
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {

    }
}
