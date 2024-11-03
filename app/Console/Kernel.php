<?php

namespace App\Console;

use App\Enums\TrendTimeRange;
use App\Helper\CacheService;
use App\ScheduleExecutor\ImgurScheduleExecutor;
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
        $schedule->command('make:post-trend all')->hourlyAt(5)->withoutOverlapping(120);
        $schedule->command('make:post-trend year')->hourlyAt(15)->withoutOverlapping(120);
        $schedule->command('make:post-trend month')->hourlyAt(25)->withoutOverlapping(120);
        $schedule->command('make:post-trend week')->hourlyAt(35)->withoutOverlapping(120);
        $schedule->command('make:post-trend day')->hourlyAt(45)->withoutOverlapping(120);

        $schedule->call(function(){
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'day', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 2]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 3]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'week', 'page' => 4]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'month', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'year', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'hot', 'range' => 'all', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 1]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 2]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 3]));
            \Http::get(route('api.public-post.index', ['sort_by' => 'new', 'page' => 4]));
        })->name('cachePosts')->everyFiveMinutes()->withoutOverlapping(120);

        $schedule->call(function(){
            Artisan::call('make:rank-report-history all');
            Artisan::call('make:rank-report-history week');
        })->name('Make Rank Report History')->dailyAt('06:15');

        $schedule->call(function(){
            app(ImgurScheduleExecutor::class)->createAlbum(30);
        })->name('Upload Imgur Albums')->hourlyAt(10)->withoutOverlapping(120);

        $schedule->call(function(){
            app(ImgurScheduleExecutor::class)->createImage(10);
        })->name('Upload Imgur Images')->everyTenMinutes()->withoutOverlapping(60);

        $schedule->call(function(){
            app(ImgurScheduleExecutor::class)->updateRemovedImage(1000);
        })->name('Update Removed Imgur Images')->hourlyAt(30)->withoutOverlapping(120);

        $schedule->call(function(){
            app(ThumbnailExecutor::class)->makeElementThumbnails(300);
        })->name('Make Thumbnails')->hourly()->withoutOverlapping(120);


        if(config('services.twitch.auto_refresh_token')){
            $schedule->command('refresh:token twitch')->name('Refresh Twitch Token')->daily();
            $schedule->command('refresh:token imgur')->name('Refresh Twitch Token')->daily();
        }

        $schedule->command('sitemap:generate')->name('Generate Sitemap')->dailyAt('05:20');

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
