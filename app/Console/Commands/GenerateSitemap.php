<?php
namespace App\Console\Commands;

use Illuminate\Console\Command;
use Spatie\Sitemap\SitemapGenerator;
use Spatie\Sitemap\Tags\Url;


class GenerateSitemap extends Command
{
    /**
     * The console command name.
     *
     * @var string
     */
    protected $signature = 'sitemap:generate';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Generate the sitemap.';

    /**
     * Execute the console command.
     *
     * @return mixed
     */
    public function handle()
    {
        SitemapGenerator::create(config('app.url'))
            ->hasCrawled(function (Url $url) {
                if ($url->path() === '' || $url->path() === '/') {
                    $url->setChangeFrequency(Url::CHANGE_FREQUENCY_HOURLY)
                        ->setPriority(1.0)
                        ->addImage(asset('/storage/og-image.jpeg'), 'Home page image');
                } elseif ($url->segment(1) === 'login' || $url->segment(1) === 'register') {
                    $url->setChangeFrequency(Url::CHANGE_FREQUENCY_MONTHLY)
                        ->setPriority(0.1);
                }

                return $url;
            })
            ->getSitemap()
            ->writeToFile(public_path('sitemap.xml'));
    }
}