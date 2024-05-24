<?php

namespace App\Console\Commands;

use App\Enums\RankReportTimeRange;
use App\Models\Post;
use App\Models\RankReport;
use App\Services\RankService;
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
        $rankService = new RankService;
        $range = $this->argument('range');
        $refresh = $this->option('refresh');
        $range = RankReportTimeRange::from($range);

        RankReport::with('post')->chunkById(1000, function ($reports) use ($rankService, $range, $refresh) {
            foreach ($reports as $report) {
                $rankService->createRankReportHistory($report, $range, $refresh);
                $this->info("Rank report history created for report id: $report->id");
            }
        });
        
        // after creating rank_report_history, update the rank of each rank_report_history
        Post::chunkById(1000, function ($posts) use ($rankService, $range) {
            foreach ($posts as $post) {
                $rankService->updateRankReportHistoryRank($post, $range);
                $this->info("Rank report history updated for post id: $post->id");
            }
        });
        
        return 0;
    }
}
