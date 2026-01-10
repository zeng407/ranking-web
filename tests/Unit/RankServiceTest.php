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

    public function test_get_rank_reports_returns_paginated_results_from_cache()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 15);

        // Create rank reports
        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
        }

        $rankService = app(RankService::class);

        // Test first page
        $page1 = $rankService->getRankReports($post, $limit = 10, $page = 1);

        $this->assertCount(10, $page1->items());
        $this->assertEquals(15, $page1->total());
        $this->assertEquals(1, $page1->currentPage());
        $this->assertEquals(2, $page1->lastPage());

        // Verify items are RankReport instances
        foreach ($page1->items() as $item) {
            $this->assertInstanceOf(RankReport::class, $item);
        }

        // Test second page
        $page2 = $rankService->getRankReports($post, $limit = 10, $page = 2);

        $this->assertCount(5, $page2->items());
        $this->assertEquals(15, $page2->total());
        $this->assertEquals(2, $page2->currentPage());
    }

    public function test_get_rank_reports_pagination_maintains_consistent_ordering()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 12);

        $expectedOrder = [];
        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
            $expectedOrder[] = $element->id;
        }

        $rankService = app(RankService::class);
        $allItems = [];

        // Collect all items across pages
        $page1 = $rankService->getRankReports($post, $limit = 5, $page = 1);
        $page2 = $rankService->getRankReports($post, $limit = 5, $page = 2);
        $page3 = $rankService->getRankReports($post, $limit = 5, $page = 3);

        foreach ([$page1, $page2, $page3] as $page) {
            foreach ($page->items() as $item) {
                $allItems[] = $item->element_id;
            }
        }

        $this->assertEquals($expectedOrder, $allItems);
    }

    public function test_get_rank_reports_excludes_deleted_elements()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 5);
        $deletedElement = $this->createElements($post, 1)->first();
        $deletedElement->delete();

        // Create reports for both active and deleted elements
        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
        }

        RankReport::create([
            'post_id' => $post->id,
            'element_id' => $deletedElement->id,
            'rank' => 6,
            'win_rate' => 50,
        ]);

        $rankService = app(RankService::class);
        $results = $rankService->getRankReports($post, $limit = 10, $page = 1);

        // Should only return 5 reports (excluding the deleted element's report)
        $this->assertCount(5, $results->items());
        $this->assertEquals(5, $results->total());
    }

    public function test_get_rank_reports_cache_miss_falls_back_to_database()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 5);

        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
        }

        $rankService = app(RankService::class);

        // Clear cache to simulate cache miss
        \Cache::forget('rank_reports_all:' . $post->id);

        // Mock CacheService::rememberRankReports to return null (cache miss)
        $this->mock(\App\Helper\CacheService::class, function ($mock) use ($post) {
            $mock->shouldReceive('rememberRankReports')
                ->with($post)
                ->andReturn(null);
        });

        $results = $rankService->getRankReports($post, $limit = 10, $page = 1);

        // Should fallback to database query
        $this->assertCount(5, $results->items());
        $this->assertEquals(5, $results->total());

        foreach ($results->items() as $item) {
            $this->assertInstanceOf(RankReport::class, $item);
        }
    }

    public function test_get_rank_reports_array_to_model_conversion()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 3);

        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
        }

        $rankService = app(RankService::class);
        $results = $rankService->getRankReports($post, $limit = 10, $page = 1);

        $this->assertCount(3, $results->items());

        foreach ($results->items() as $item) {
            // Verify it's a RankReport instance
            $this->assertInstanceOf(RankReport::class, $item);

            // Verify the model exists flag is set (indicating it was converted from array)
            $this->assertTrue($item->exists);

            // Verify essential attributes are preserved
            $this->assertIsInt($item->id);
            $this->assertIsInt($item->element_id);
            $this->assertTrue($item->rank === null || is_int($item->rank));
            $this->assertTrue($item->win_rate === null || is_numeric($item->win_rate));
        }
    }

    public function test_get_rank_reports_cached_vs_database_consistency()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 10);

        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 5),
            ]);
        }

        $rankService = app(RankService::class);

        // Get results from cache (first call populates cache)
        $cachedResults = $rankService->getRankReports($post, $limit = 10, $page = 1);
        $cachedItems = collect($cachedResults->items())->map(function ($item) {
            return [
                'id' => $item->id,
                'element_id' => $item->element_id,
                'rank' => $item->rank,
                'win_rate' => $item->win_rate,
            ];
        })->toArray();

        // Clear cache and get from database directly
        \Cache::forget('rank_reports_all:' . $post->id);

        $this->mock(\App\Helper\CacheService::class, function ($mock) use ($post) {
            $mock->shouldReceive('rememberRankReports')
                ->with($post)
                ->andReturn(null);
        });

        $dbResults = $rankService->getRankReports($post, $limit = 10, $page = 1);
        $dbItems = collect($dbResults->items())->map(function ($item) {
            return [
                'id' => $item->id,
                'element_id' => $item->element_id,
                'rank' => $item->rank,
                'win_rate' => $item->win_rate,
            ];
        })->toArray();

        // Verify consistency
        $this->assertEquals($cachedItems, $dbItems);
    }

    public function test_get_rank_reports_handles_empty_post()
    {
        $post = $this->createPost();

        $rankService = app(RankService::class);
        $results = $rankService->getRankReports($post, $limit = 10, $page = 1);

        $this->assertCount(0, $results->items());
        $this->assertEquals(0, $results->total());
    }

    public function test_get_rank_reports_respects_limit_parameter()
    {
        $post = $this->createPost();
        $elements = $this->createElements($post, 25);

        foreach ($elements as $index => $element) {
            RankReport::create([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank' => $index + 1,
                'win_rate' => 90 - ($index * 2),
            ]);
        }

        $rankService = app(RankService::class);

        // Test with different limits
        $results5 = $rankService->getRankReports($post, $limit = 5, $page = 1);
        $this->assertCount(5, $results5->items());

        $results15 = $rankService->getRankReports($post, $limit = 15, $page = 1);
        $this->assertCount(15, $results15->items());

        // Verify total is consistent
        $this->assertEquals(25, $results5->total());
        $this->assertEquals(25, $results15->total());
    }
}
