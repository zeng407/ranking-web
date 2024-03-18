<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class CreateCommentsTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        Schema::create('comments', function (Blueprint $table) {
            $table->id();
            $table->foreignid('user_id')->nullable()->constrained()->setNullOnDelete();
            $table->string('anonymous_id')->index();
            $table->string('nickname', 30);
            $table->string('content', 512);
            $table->text('label')->nullable();
            $table->ipAddress('ip');
            $table->boolean('anonymous_mode')->default(false);
            $table->string('delete_hash', 64);
            $table->timestamp('edited_at')->nullable();
            $table->softDeletes();
            $table->timestamps();
        });
    }

    /**
     * Reverse the migrations.
     *
     * @return void
     */
    public function down()
    {
        Schema::dropIfExists('comments');
    }
}
