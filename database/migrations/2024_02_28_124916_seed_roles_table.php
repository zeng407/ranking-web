<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

class SeedRolesTable extends Migration
{
    /**
     * Run the migrations.
     *
     * @return void
     */
    public function up()
    {
        $roles = [
            ['name' => 'Administrator', 'slug' => 'admin', 'description' => 'This role has full access to the application'],
            ['name' => 'Banned', 'slug' => 'banned', 'description' => 'This role is for banned users'],
        ];

        foreach ($roles as $role) {
            \App\Models\Role::firstOrCreate(['slug' => $role['slug']], $role);
        }

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
