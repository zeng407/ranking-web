<?php
namespace App\Console\Commands;

use Illuminate\Console\Command;
use Spatie\Sitemap\SitemapGenerator;
use Spatie\Sitemap\Tags\Url;
use App\Models\Post;
use Spatie\Crawler\Crawler;


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
        $sitemap = SitemapGenerator::create(config('app.url'))
            ->configureCrawler(function (Crawler $crawler) {
                $crawler->setMaximumDepth(0);
            })
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
            ->getSitemap();

        Post::setEagerLoads([])
            ->where('created_at', '>=', now()->subMonths(3))
            ->eachById(function (Post $post) use ($sitemap) {
                $sitemap->add(route('game.show', $post))
                    ->add(route('game.rank', $post));
            });

        $sitemap->writeToFile(public_path('sitemap.xml'));
    }
}
