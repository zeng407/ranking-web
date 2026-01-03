<?php

namespace Tests\Unit;

use App\Enums\RankType;
use App\Models\Element;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Services\RankService;
use Carbon\Carbon;
use Tests\TestCase;
use Tests\TestHelper;

class RankServiceTest extends TestCase
{
    use TestHelper;

    public function test_create_rank_reports_builds_consistent_rankings()
    {
        Carbon::setTestNow(Carbon::parse('2024-01-01'));

        $post = $this->createPost();
        $elements = $this->createElements($post, 3);
        $deletedElement = $this->createElements($post, 1)->first();
        $deletedElement->delete();

        $staleReport = RankReport::create([
            'post_id' => $post->id,
            'element_id' => $deletedElement->id,
        ]);

        $this->seedRankMetrics($post, $elements);

        app(RankService::class)->createRankReports($post);

        $this->assertDatabaseHas('rank_reports', [
            'post_id' => $post->id,
            'element_id' => $elements[0]->id,
            'final_win_position' => 1,
            'final_win_rate' => 90.00,
        ]);

        $this->assertDatabaseHas('rank_reports', [
            'post_id' => $post->id,
            'element_id' => $elements[1]->id,
            'final_win_position' => 2,
            'final_win_rate' => 80.00,
        ]);

        $this->assertDatabaseHas('rank_reports', [
            'post_id' => $post->id,
            'element_id' => $elements[2]->id,
            'win_position' => 2,
            'win_rate' => 55.00,
        ]);

        $this->assertEquals([
            $elements[1]->id,
            $elements[2]->id,
            $elements[0]->id,
        ], RankReport::where('post_id', $post->id)
            ->orderBy('rank')
            ->pluck('element_id')
            ->toArray());

        $this->assertSoftDeleted('rank_reports', ['id' => $staleReport->id]);
        $this->assertEquals(3, RankReport::where('post_id', $post->id)->count());
    }

    private function seedRankMetrics(Post $post, $elements): void
    {
        $championMetrics = [90.00, 80.00, 70.00];
        $pkMetrics = [50.00, 60.00, 55.00];
        $winCounts = [9, 7, 5];

        foreach ($elements as $index => $element) {
            $this->createRankEntry($post, $element, RankType::CHAMPION, $championMetrics[$index], $winCounts[$index]);
            $this->createRankEntry($post, $element, RankType::PK_KING, $pkMetrics[$index], $winCounts[$index]);
        }
    }

    private function createRankEntry(Post $post, Element $element, string $type, float $rate, int $wins): Rank
    {
        return Rank::create([
            'post_id' => $post->id,
            'element_id' => $element->id,
            'rank_type' => $type,
            'record_date' => today(),
            'win_count' => $wins,
            'round_count' => max($wins, 1),
            'win_rate' => $rate,
        ]);
    }
}
