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

class RankService
{
    public function getRankReports(Post $post, $limit = 10, $page = null)
    {
        $reports = RankReport::where('post_id', $post->id)
            ->whereHas('element', function ($query) {
                $query->whereNull('deleted_at');
            })
            ->orderByRaw('ISNULL(`rank`)')
            ->orderBy('rank')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
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

    public function getRankReportHistoryByElement(Post $post, Element $element, RankReportTimeRange $timeRange, $limit = 10, $page = null)
    {
        $reports = RankReportHistory::where('post_id', $post->id)
            ->where('element_id', $element->id)
            ->where('time_range', $timeRange)
            ->orderByDesc('start_date')
            ->paginate($limit, ['*'], 'page', $page);

        return $reports;
    }

    public function createElementRank(Post $post, Element $element)
    {
        $completeGameRounds = Game::join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('games.post_id', $post->id)
            ->where(function($subQuery) use ($element) {
                $subQuery->where('game_1v1_rounds.winner_id', $element->id)
                    ->orWhere('game_1v1_rounds.loser_id', $element->id);
            })
            ->whereNotNull('games.completed_at')
            ->count();

        if ($completeGameRounds) {
            $championCount = Game::where('games.post_id', $post->id)
                ->whereHas('game_1v1_rounds', function ($query) use ($element) {
                    $query->where('remain_elements', 1)
                        ->where('winner_id', $element->id);
                })
                ->count();

            if ($championCount) {
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
        }

        $winCount = Game::where('games.post_id', $post->id)
            ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('game_1v1_rounds.winner_id', $element->id)
            ->count();
        $loseCount = Game::where('games.post_id', $post->id)
            ->join('game_1v1_rounds', 'game_1v1_rounds.game_id', '=', 'games.id')
            ->where('game_1v1_rounds.loser_id', $element->id)
            ->count();

        $rounds = $winCount + $loseCount;
        if ($rounds > 0) {
            if ($winCount) {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => $winCount,
                    'round_count' => $rounds,
                    'win_rate' => $winCount / $rounds * 100
                ]);
            } else {
                Rank::updateOrCreate([
                    'post_id' => $post->id,
                    'element_id' => $element->id,
                    'rank_type' => RankType::PK_KING,
                    'record_date' => today(),
                ], [
                    'win_count' => 0,
                    'round_count' => $rounds,
                    'win_rate' => 0
                ]);
            }
        }
    }

    public function createRankReports(Post $post)
    {
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

        // [Fix 1] 取得原始排序的 List (不要在這裡 keyBy，保持 0, 1, 2 的索引)
        $championRanksList = (clone $baseRankQuery)
            ->where('rank_type', RankType::CHAMPION)
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->get();

        $pkRanksList = (clone $baseRankQuery)
            ->where('rank_type', RankType::PK_KING)
            ->orderByDesc('win_rate')
            ->orderByDesc('win_count')
            ->get();

        // 格式: [element_id => rank_position]
        // 這樣查詢名次的時間複雜度是 O(1)，非常快
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
            ->visible()
            ->get()
            ->keyBy('element_id');

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

            // [Fix 3] 直接從 Map 取得名次，如果沒有就用舊的，再沒有就是 null
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

        // [Fix 4] 計算總排名 (Rank)
        // 這裡維持原樣，使用 usort 進行記憶體內排序
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

        // ==========================================
        // 第二階段：寫入 (Transaction)
        // ==========================================
        if (!empty($upsertData)) {
            DB::transaction(function () use ($post, $upsertData) {
                RankReport::where('post_id', $post->id)
                    ->whereHas('element', function ($query) {
                        $query->whereNotNull('deleted_at');
                    })
                    ->update(['hidden' => true]);

                $chunks = array_chunk($upsertData, 500);
                foreach ($chunks as $chunk) {
                    RankReport::upsert(
                        $chunk,
                        ['post_id', 'element_id'],
                        ['final_win_position', 'final_win_rate', 'win_position', 'win_rate', 'rank', 'updated_at']
                    );
                }
            });
        }

        \Log::info("end update post [{$post->id}] rank report [{$post->title}]");
    }

    private function applyRankUpdates($ranks, $rankReports, string $positionField, string $rateField): void
    {
        $counter = 0;
        foreach ($ranks as $rank) {
            $counter++;
            $report = $rankReports->get($rank->element_id);

            if (!$report) {
                $report = new RankReport([
                    'post_id' => $rank->post_id,
                    'element_id' => $rank->element_id,
                ]);
                $rankReports->put($rank->element_id, $report);
            }

            $report->setAttribute($positionField, $counter);
            $report->setAttribute($rateField, $rank->win_rate);
            $report->save();
        }
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
}
