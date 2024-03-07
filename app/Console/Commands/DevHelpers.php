<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;

class DevHelpers extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'ide-helper:run';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'auto-generate ide-helper files';

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        if (config('app.env') === 'local') {
            $this->info('generating ide helper files...');
            $this->call('ide-helper:generate');
            $this->call('ide-helper:meta');
            $this->call('ide-helper:models', ['--nowrite' => true, '--write-mixin' => true]);
            $this->info('... generating ide helper files done');
        }
        return 0;
    }
}
