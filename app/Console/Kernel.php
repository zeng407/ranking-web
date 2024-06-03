<?php

namespace App\Console;

use App\ScheduleExecutor\PostTrendScheduleExecutor;
use App\ScheduleExecutor\ThumbnailExecutor;
use Artisan;
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
        $schedule->call(function(){
            //todo fix memory exhausted
            app(PostTrendScheduleExecutor::class)->createPostTrends();
        })->name('createPostTrend')->hourlyAt(5)->withoutOverlapping();

        $schedule->call(function(){
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'day']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 2]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 3]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 4]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'month']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'year']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'all']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new']));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 2]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 3]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 4]));
        })->name('cachePosts')->everyFiveMinutes()->withoutOverlapping();

        $schedule->call(function(){
            Artisan::call('make:rank-report-history all');
            Artisan::call('make:rank-report-history week');
        })->name('Make Rank Report History')->dailyAt('06:15');

        $schedule->call(function(){
            app(ThumbnailExecutor::class)->makeElementThumbnails(300);
        })->name('Make Thumbnails')->hourly()->withoutOverlapping();


        if(config('services.twitch.auto_refresh_token')){
            $schedule->command('refresh:token twitch')->name('Refresh Twitch Token')->daily();
        }

        $schedule->command('sitemap:generate')->name('Generate Sitemap')->dailyAt('00:20');

        if(app()->isLocal()){
            $schedule->command('telescope:prune')->daily();
        }

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
