<?php


namespace App\Services;

use App\Enums\RankReportTimeRange;
use App\Enums\RankType;
use App\Helper\CacheService;
use App\Jobs\UpdateRankForReportHistory;
use App\Models\Element;
use App\Models\Game;
use App\Models\GameElement;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Models\RankReportHistory;
use App\Services\Builders\RankReportHistoryBuilder;
use DB;
use Illuminate\Database\QueryException;

class RankService
{
    public function getRankReports(Post $post, $limit = 10, $page = null)
    {
        $allReports = CacheService::rememberRankReports($post);

        if ($allReports && !empty($allReports)) {
            if (is_array($allReports)) {
                $allReports = collect($allReports)->map(function ($item) {
                    $report = new RankReport();
                    foreach ((array) $item as $key => $value) {
                        $report->setAttribute($key, $value);
                    }
                    $report->exists = true;
                    return $report;
                });
            }

            $page = $page ?: request()->input('page', 1);
            $offset = ($page - 1) * $limit;
            $items = $allReports->slice($offset, $limit)->values();

            return new \Illuminate\Pagination\LengthAwarePaginator(
                $items,
                $allReports->count(),
                $limit,
                $page,
                ['path' => request()->url(), 'query' => request()->query()]
            );
        }

        // Fallback if cache fails
        return RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByRaw('ISNULL(`rank`)')
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);
    }

    public function getRankReportByElement(Post $post, Element $element): ?RankReport
    {
        return RankReport::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->first();
    }

    public function getRanksByElementTitle(Post $post, string $title)
    {
        return RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) use ($title) {
                $query->where('title', 'like', '%' . $title . '%');
            })
            ->limit(10)
            ->orderBy('rank')
            ->get();
    }

    public function getRankPosition(Post $post, Element $element): ?int
    {
        $report = $this->getRankReportByElement($post, $element);

        return $report?->rank;
    }

    public function getRankWeeklyReport(Post $post, $limit = 10, $page = null)
    {
        $latestDate = RankReportHistory::where('post_id', $post->id)
            ->where('time_range', RankReportTimeRange::WEEK)
            ->max('start_date');
        $reports = $post->rank_report_histories()->with('element')
            ->where('time_range', RankReportTimeRange::WEEK)
            ->where('start_date', $latestDate)
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankReportHistoryByRankReport(RankReport $rankReport, RankReportTimeRange $timeRange, $limit = 10, $page = null)
    {
        $reports = $rankReport->rank_report_histories()
            ->where('time_range', $timeRange)
            ->orderByDesc('start_date')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function getRankReportHistoryByElement(Post $post, $elementId, RankReportTimeRange $timeRange, $limit = 10, $page = null)
    {
        $reports = RankReportHistory::where('post_id', $post->id)
            ->where('element_id', $elementId)
            ->where('time_range', $timeRange)
            ->orderByDesc('start_date')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function createElementRank(Post $post, Element $element)
    {
        $stats = CacheService::getElementRankStats($post->id, $element->id);

        // --- Champion Stats (Completed Games) ---
        // We track parts separately to support incremental updates
        // 1. champion_win_rounds (Winner in rounds of completed games)
        // 2. champion_lose_rounds (Loser in rounds of completed games)
        // 3. champion_game_wins (Winner of game where remain_elements=1)

        $championMaxWinId = $stats['champion_max_win_id'] ?? 0;
        $championMaxLoseId = $stats['champion_max_lose_id'] ?? 0;

        $statChampionRoundWins = $stats['champion_round_wins'] ?? 0;
        $statChampionRoundLoses = $stats['champion_round_loses'] ?? 0;
        $statChampionGameWins = $stats['champion_game_wins'] ?? 0;

        // Query new WINNER records for Champion Stats (Aggregate directly in DB)
        $newChampionWinStats = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->whereNotNull('games.completed_at')
            ->where('game_1v1_rounds.winner_id', $element->id)
            ->where('game_1v1_rounds.id', '>', $championMaxWinId)
            ->selectRaw('COUNT(*) as count, MAX(game_1v1_rounds.id) as max_id, SUM(CASE WHEN remain_elements = 1 THEN 1 ELSE 0 END) as champion_count')
            ->first();

        if ($newChampionWinStats->count > 0) {
            $statChampionRoundWins += $newChampionWinStats->count;
            $statChampionGameWins += (int) $newChampionWinStats->champion_count;
            $championMaxWinId = max($championMaxWinId, $newChampionWinStats->max_id);
        }

        // Query new LOSER records for Champion Stats (Aggregate directly in DB)
        $newChampionLoseStats = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->whereNotNull('games.completed_at')
            ->where('game_1v1_rounds.loser_id', $element->id)
            ->where('game_1v1_rounds.id', '>', $championMaxLoseId)
            ->selectRaw('COUNT(*) as count, MAX(game_1v1_rounds.id) as max_id')
            ->first();

        if ($newChampionLoseStats->count > 0) {
            $statChampionRoundLoses += $newChampionLoseStats->count;
            $championMaxLoseId = max($championMaxLoseId, $newChampionLoseStats->max_id);
        }

        // Calculate Totals
        $completeGameRounds = $statChampionRoundWins + $statChampionRoundLoses;
        $championCount = $statChampionGameWins;

        if ($completeGameRounds > 0 && $championCount > 0) {
            Rank::updateOrCreate([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank_type' => RankType::CHAMPION,
                'record_date' => today(),
            ], [
                'win_count' => $championCount,
                'round_count' => $completeGameRounds,
                'win_rate' => $championCount / $completeGameRounds * 100
            ]);
        }

        // --- PK King Stats (All Games) ---
        // 1. pk_win_count
        // 2. pk_lose_count

        $pkMaxWinId = $stats['pk_max_win_id'] ?? 0;
        $pkMaxLoseId = $stats['pk_max_lose_id'] ?? 0;

        $statPkWins = $stats['pk_win_count'] ?? 0;
        $statPkLoses = $stats['pk_lose_count'] ?? 0;

        // Query new WIN records for PK Stats (Aggregate directly in DB)
        $newPkWinStats = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->where('game_1v1_rounds.winner_id', $element->id)
            ->where('game_1v1_rounds.id', '>', $pkMaxWinId)
            ->selectRaw('COUNT(*) as count, MAX(game_1v1_rounds.id) as max_id')
            ->first();

        if ($newPkWinStats->count > 0) {
            $statPkWins += $newPkWinStats->count;
            $pkMaxWinId = max($pkMaxWinId, $newPkWinStats->max_id);
        }

        // Query new LOSE records for PK Stats (Aggregate directly in DB)
        $newPkLoseStats = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->where('game_1v1_rounds.loser_id', $element->id)
            ->where('game_1v1_rounds.id', '>', $pkMaxLoseId)
            ->selectRaw('COUNT(*) as count, MAX(game_1v1_rounds.id) as max_id')
            ->first();

        if ($newPkLoseStats->count > 0) {
            $statPkLoses += $newPkLoseStats->count;
            $pkMaxLoseId = max($pkMaxLoseId, $newPkLoseStats->max_id);
        }

        // Update Cache
        $newStats = [
            'champion_max_win_id' => $championMaxWinId,
            'champion_max_lose_id' => $championMaxLoseId,
            'champion_round_wins' => $statChampionRoundWins,
            'champion_round_loses' => $statChampionRoundLoses,
            'champion_game_wins' => $statChampionGameWins,
            'pk_max_win_id' => $pkMaxWinId,
            'pk_max_lose_id' => $pkMaxLoseId,
            'pk_win_count' => $statPkWins,
            'pk_lose_count' => $statPkLoses,
        ];

        CacheService::putElementRankStats($post->id, $element->id, $newStats);

        $winCount = $statPkWins;
        $loseCount = $statPkLoses;

        // Update PK King Rank
        $rounds = $winCount + $loseCount;
        if ($rounds > 0) {
            $winRate = $winCount ? ($winCount / $rounds * 100) : 0;

            Rank::updateOrCreate([
                'post_id' => $post->id,
                'element_id' => $element->id,
                'rank_type' => RankType::PK_KING,
                'record_date' => today(),
            ], [
                'win_count' => $winCount,
                'round_count' => $rounds,
                'win_rate' => $winRate
            ]);
        }
    }

    public function createRankReports(Post $post)
    {
        $t0 = microtime(true);
        \Log::info("start update post [{$post->id}] rank report [{$post->title}]");

        // ==========================================
        // 第一階段：準備與計算
        // ==========================================
        $baseRankQuery = Rank::select(['element_id', 'rank_type', 'win_rate', 'win_count'])
            ->where('post_id', $post->id)
            ->where('record_date', today())
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            });

        $tBaseFetchStart = microtime(true);
        $baseRanks = $baseRankQuery->get();
        $tBaseFetchEnd = microtime(true);

        // 取得原始排序的 List (本地排序)
        $tChampionProcessStart = microtime(true);
        $championRanksList = $baseRanks
            ->where('rank_type', RankType::CHAMPION)
            ->values()
            ->all();
        usort($championRanksList, function ($a, $b) {
            if ($b->win_rate != $a->win_rate) {
                return $b->win_rate <=> $a->win_rate;
            }
            return $b->win_count <=> $a->win_count;
        });
        $tChampionProcessEnd = microtime(true);

        $tPkProcessStart = microtime(true);
        $pkRanksList = $baseRanks
            ->where('rank_type', RankType::PK_KING)
            ->values()
            ->all();
        usort($pkRanksList, function ($a, $b) {
            if ($b->win_rate != $a->win_rate) {
                return $b->win_rate <=> $a->win_rate;
            }
            return $b->win_count <=> $a->win_count;
        });
        $tPkProcessEnd = microtime(true);

        $tExistingStart = microtime(true);

        // 將 element_id 映射到對應資料／名次：
        // - $championMap / $pkMap      : [element_id => Rank]，用來 O(1) 查詢數值 (例如 win_rate)
        // - $championPosMap / $pkPosMap: [element_id => rank_position]，用來 O(1) 查詢名次 (position)
        $championMap = [];     // 用來查數值 (win_rate)
        $championPosMap = [];  // 用來查名次 (position)
        foreach ($championRanksList as $index => $rank) {
            $championMap[$rank->element_id] = $rank;
            $championPosMap[$rank->element_id] = $index + 1; // 名次 = 索引 + 1
        }

        $pkMap = [];
        $pkPosMap = [];
        foreach ($pkRanksList as $index => $rank) {
            $pkMap[$rank->element_id] = $rank;
            $pkPosMap[$rank->element_id] = $index + 1;
        }

        // 取得現有的 Reports
        $existingReports = RankReport::where('post_id', $post->id)
            ->get()
            ->keyBy('element_id');

        $tAssembleStart = microtime(true);

        // 組裝資料
        $upsertData = [];
        $now = now();

        $allElementIds = $existingReports->keys()
            ->merge(array_keys($championMap))
            ->merge(array_keys($pkMap))
            ->unique();

        foreach ($allElementIds as $elementId) {
            $report = $existingReports->get($elementId);

            // 從 Map 取得資料
            $champion = $championMap[$elementId] ?? null;
            $pk = $pkMap[$elementId] ?? null;

            // 直接從 Map 取得名次，如果沒有就用舊的，再沒有就是 null
            $finalWinPosition = $championPosMap[$elementId] ?? ($report ? $report->final_win_position : null);
            $winPosition = $pkPosMap[$elementId] ?? ($report ? $report->win_position : null);

            $finalWinRate = $champion ? $champion->win_rate : ($report ? $report->final_win_rate : 0);
            $winRate = $pk ? $pk->win_rate : ($report ? $report->win_rate : 0);

            $data = [
                'post_id' => $post->id,
                'element_id' => $elementId,
                'final_win_position' => $finalWinPosition,
                'final_win_rate' => $finalWinRate,
                'win_position' => $winPosition,
                'win_rate' => $winRate,
                'updated_at' => $now,
            ];

            if (!$report) {
                $data['created_at'] = $now;
            } else {
                $data['created_at'] = $report->created_at;
            }

            $upsertData[] = $data;
        }

        // 計算總排名 (Rank)
        // 這裡維持原樣，使用 usort 進行記憶體內排序
        $tSortStart = microtime(true);
        usort($upsertData, function ($a, $b) {
            // 邏輯：優先比較 win_rate，若相同則比較 final_win_rate
            if ($b['win_rate'] != $a['win_rate']) {
                return $b['win_rate'] <=> $a['win_rate'];
            }
            return $b['final_win_rate'] <=> $a['final_win_rate'];
        });

        // 填入 Rank 欄位
        foreach ($upsertData as $index => &$row) {
            $row['rank'] = $index + 1;
        }
        unset($row);

        $tStage1End = microtime(true);

        // ==========================================
        // 第二階段：寫入 (Transaction)
        // ==========================================
        $deletedElementIds = \DB::table('elements')
            ->join('post_elements', 'post_elements.element_id', '=', 'elements.id')
            ->where('post_elements.post_id', $post->id)
            ->whereNotNull('elements.deleted_at')
            ->pluck('elements.id');

        $tTxStart = microtime(true);

        if ($deletedElementIds->isNotEmpty()) {
            RankReport::where('post_id', $post->id)
                ->whereIn('element_id', $deletedElementIds)
                ->update(['hidden' => true]);
        }

        if (!empty($upsertData)) {
            $orderedUpsertData = $upsertData;
            usort($orderedUpsertData, function ($a, $b) {
                return $a['element_id'] <=> $b['element_id'];
            });

            $chunks = array_chunk($orderedUpsertData, 200);
            foreach ($chunks as $chunk) {
                $this->upsertRankReportsWithRetry($chunk);
            }
        }

        $tTxEnd = microtime(true);

        \Log::info('rank report timing', [
            'post_id' => $post->id,
            'stage_base_fetch_ms' => ($tBaseFetchEnd - $tBaseFetchStart) * 1000,
            'stage_champion_process_ms' => ($tChampionProcessEnd - $tChampionProcessStart) * 1000,
            'stage_pk_process_ms' => ($tPkProcessEnd - $tPkProcessStart) * 1000,
            'stage_existing_fetch_ms' => ($tAssembleStart - $tExistingStart) * 1000,
            'stage_assemble_ms' => ($tSortStart - $tAssembleStart) * 1000,
            'stage_sort_ms' => ($tStage1End - $tSortStart) * 1000,
            'stage_write_ms' => ($tTxEnd - $tTxStart) * 1000,
            'total_ms' => ($tTxEnd - $t0) * 1000,
        ]);

        \Log::info("end update post [{$post->id}] rank report [{$post->title}]");
    }

    public function createRankReportHistory(RankReport $rankReport, RankReportTimeRange $timeRange, $refresh = false, $start = null)
    {
        $builder = new RankReportHistoryBuilder;
        return $builder->setRankReport($rankReport)
            ->setStartAt($start)
            ->setRange($timeRange)
            ->setRefresh($refresh)
            ->build();
    }

    public function updateRankReportHistoryRank(Post $post, RankReportTimeRange $timeRange)
    {
        $dates = CacheService::pullRankHistoryNeededUpdateDatesCache($post->id, $timeRange);
        if (empty($dates)) {
            return;
        }
        foreach ($dates as $date) {
            UpdateRankForReportHistory::dispatch($post->id, $timeRange, $date);
        }
    }

    /**
     * Upsert rank reports with deadlock retries to reduce 1213/40001 failures.
     */
    private function upsertRankReportsWithRetry(array $chunk, int $maxAttempts = 3): void
    {
        $attempt = 0;
        while ($attempt < $maxAttempts) {
            try {
                RankReport::upsert(
                    $chunk,
                    ['post_id', 'element_id'],
                    ['final_win_position', 'final_win_rate', 'win_position', 'win_rate', 'rank', 'updated_at']
                );
                return;
            } catch (QueryException $e) {
                $code = (string) $e->getCode();
                if (!in_array($code, ['40001', '1213'])) {
                    throw $e;
                }

                $attempt++;
                if ($attempt >= $maxAttempts) {
                    throw $e;
                }

                usleep(random_int(100000, 400000));
            }
        }
    }
}
