<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;

class CreateAdmin extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'make:admin {email}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Create a new admin role for a user';

    /**
     * Create a new command instance.
     *
     * @return void
     */
    public function __construct()
    {
        parent::__construct();
    }

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        $email = $this->argument('email');
        $user = \App\Models\User::where('email', $email)->first();
        if (!$user) {
            $this->error('User not found');
            return 1;
        }
        $user->roles()->syncWithoutDetaching(find_role_id(\App\Enums\Role::ADMIN));

        $this->info('Admin role created for user ' . $email);
        return 0;
    }
}
