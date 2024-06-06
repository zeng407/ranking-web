<?php

namespace App\Console\Commands;

use App\ScheduleExecutor\PostTrendScheduleExecutor;
use Illuminate\Console\Command;

class CreatePostTrend extends Command
{
    protected PostTrendScheduleExecutor $postTrendScheduleExecutor;
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'make:post-trend';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Create post trend';

    /**
     * Create a new command instance.
     *
     * @return void
     */
    public function __construct(PostTrendScheduleExecutor $postTrendScheduleExecutor)
    {
        parent::__construct();
        $this->postTrendScheduleExecutor = $postTrendScheduleExecutor;
    }

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        $this->postTrendScheduleExecutor->createPostTrends();
        return 0;
    }
}
