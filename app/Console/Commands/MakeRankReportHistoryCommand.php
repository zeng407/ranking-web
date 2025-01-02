<?php

namespace App\Console\Commands;

use App\Enums\RankReportTimeRange;
use App\Jobs\CreateAndUpdateRankHistory;
use App\Models\Post;
use Illuminate\Console\Command;

class MakeRankReportHistoryCommand extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'make:rank-report-history {range} {--refresh}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Create a new rank report history for the given time range.';

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
        $range = $this->argument('range');
        $range = RankReportTimeRange::from($range);

        $refresh = $this->option('refresh');

        Post::setEagerLoads([])->chunkById(300, function ($posts)use($refresh){
            foreach ($posts as $post) {
                CreateAndUpdateRankHistory::dispatch(
                    $post,
                    $refresh
                );
                $this->info("Rank report history created for post id: $post->id");
            }
        });

        return 0;
    }
}
