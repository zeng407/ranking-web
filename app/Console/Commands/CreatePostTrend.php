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
    protected $signature = 'make:post-trend {range}';

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
        $range = $this->argument('range');
        $this->info("Create post trend for range: $range");

        switch ($range) {
            case 'all':
                $this->postTrendScheduleExecutor->createAllPostTrends();
                break;
            case 'year':
                $this->postTrendScheduleExecutor->createYearPostTrends();
                break;
            case 'month':
                $this->postTrendScheduleExecutor->createMonthPostTrends();
                break;
            case 'week':
                $this->postTrendScheduleExecutor->createWeekPostTrends();
                break;
            case 'day':
                $this->postTrendScheduleExecutor->createTodayPostTrends();
                break;
            default:
                $this->error('Invalid range');
                break;
        }
        return 0;
    }
}
