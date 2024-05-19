<?php

namespace App\Console\Commands;

use App\Jobs\MakeVideoThumbnail;
use Illuminate\Console\Command;

class MakeVideoThumbnailCommand extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'make:video-thumbnail {elementId}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Make video thumbnail from video element id.';

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
        $elementId = $this->argument('elementId');
        MakeVideoThumbnail::dispatch($elementId);
        return 0;
    }
}
