<?php

namespace App\Console;

use App\ScheduleExecutor\PostTrendScheduleExecutor;
use App\ScheduleExecutor\RankScheduleExecutor;
use Illuminate\Console\Scheduling\Schedule;
use Illuminate\Foundation\Console\Kernel as ConsoleKernel;

class Kernel extends ConsoleKernel
{
    /**
     * Define the application's command schedule.
     *
     * @param  \Illuminate\Console\Scheduling\Schedule  $schedule
     * @return void
     */
    protected function schedule(Schedule $schedule)
    {
        // $schedule->command('inspire')->hourly();
//        $schedule->call(fn() => \Log::debug("schedule call"))->everyMinute();
        $schedule->call(function(){
           app(RankScheduleExecutor::class)->createRankReports();
        })->name('createRankReport')->everyFiveMinutes()->withoutOverlapping();

        $schedule->call(function(){
            app(PostTrendScheduleExecutor::class)->createPostTrends();
        })->name('createPostTrend')->everyThreeHours()->withoutOverlapping();
    }

    /**
     * Register the commands for the application.
     *
     * @return void
     */
    protected function commands()
    {
        $this->load(__DIR__.'/Commands');

        require base_path('routes/console.php');
    }
}
