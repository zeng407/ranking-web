<?php

namespace App\Services\Builders;

use App\Enums\RankReportTimeRange;
use App\Enums\RankType;
use App\Models\Element;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\Rank;
use App\Models\RankReport;
use App\Models\RankReportHistory;

class RankReportHistoryBuilder
{
    protected RankReport $report;
    protected RankReportTimeRange $range;

    protected $refresh = false;

    public function setRankReport(RankReport $report): self
    {
        $this->report = $report;
        return $this;
    }

    public function setRange(RankReportTimeRange $range): self
    {
        $this->range = $range;
        return $this;
    }

    public function setRefresh(bool $refresh): self
    {
        $this->refresh = $refresh;
        return $this;
    }

    public function build()
    {
        switch ($this->range) {
            case RankReportTimeRange::WEEK:
                return $this->buildWeek();
            case RankReportTimeRange::MONTH:
                return $this->buildMonth();
            case RankReportTimeRange::YEAR:
                return $this->buildYear();
            case RankReportTimeRange::ALL:
                return $this->buildAll();
        }
    }

    protected function buildAll()
    {
        if($this->refresh) {
            $this->deleteHistory(RankReportTimeRange::ALL);
        }

        $start = $this->getLastStartDate(RankReportTimeRange::ALL);

        // 錯誤容忍度，如果有遺漏的資料，最多回補 1 天
        // fail tolerance, if there are missing data, at most fill in 3 days
        $start = carbon($start)->subDays(1);

        $lastRecord = $this->getLastRankRecord($start, RankType::PK_KING);
        $sumWinCount = $lastRecord->win_count ?? 0;
        $sumLoseCount = $lastRecord ? $lastRecord->round_count - $lastRecord->win_count : 0;
        $sumRounds = $lastRecord->round_count ?? 0;

        $lastRecord = $this->getLastRankRecord($start, RankType::CHAMPION);
        $championCount = $lastRecord->win_count ?? 0;
        $gameCompleteCount = $lastRecord->round_count ?? 0;

        $rankRecords = $this->getRankRecords($start);
        $timeline = carbon($start);
        while($timeline->lte(today()->endOfDay())) {
            $rankPKRecord = $rankRecords->where('record_date', carbon($timeline)->toDateString())
                ->where('rank_type', RankType::PK_KING)
                ->first();
            if($rankPKRecord){
                $sumWinCount = $rankPKRecord->win_count;
                $sumLoseCount = $rankPKRecord->round_count - $rankPKRecord->win_count;
                $sumRounds = $rankPKRecord->round_count;
            }

            $rankChampionRecord = $rankRecords->where('record_date', carbon($timeline)->toDateString())
                ->where('rank_type', RankType::CHAMPION)
                ->first();
            if($rankChampionRecord){
                $championCount = $rankChampionRecord->win_count;
                $gameCompleteCount = $rankChampionRecord->round_count;
            }

            $winRate = $sumRounds > 0 ? $sumWinCount / $sumRounds * 100 : 0;
            $championRate = $gameCompleteCount > 0 ? $championCount / $gameCompleteCount * 100 : 0;

            RankReportHistory::updateOrCreate([
                'element_id' => $this->report->element_id,
                'post_id' => $this->report->post_id,
                'rank_report_id' => $this->report->id,
                'time_range' => RankReportTimeRange::ALL,
                'start_date' => $timeline->toDateString(),
            ], [
                'rank' => 0, // we mark the rank as 0, then update the rank later
                'win_rate' => $winRate,
                'win_count' => $sumWinCount,
                'lose_count' => $sumLoseCount,
                'champion_count' => $championCount,
                'game_complete_count' => $gameCompleteCount,
                'champion_rate' => $championRate
            ]);

            $timeline->addDay();
        }
    }

    protected function buildWeek()
    {
        if($this->refresh) {
            $this->deleteHistory(RankReportTimeRange::WEEK);
        }

        $start = $this->getLastStartDate(RankReportTimeRange::WEEK);
        $start = carbon($start)->startOfWeek();

        // 錯誤容忍度，如果有遺漏的資料，最多回補 1 天
        // fail tolerance, if there are missing data, at most fill in 3 days
        // $start = carbon($start)->subDay(1);

        $rankRecords = $this->getRankRecords($start);


        $lastChampionRecord = $this->getLastRankRecord($start, RankType::CHAMPION);
        $lastChampionCount = $lastChampionRecord->win_count ?? 0;
        $lastGameCompleteCount = $lastChampionRecord->round_count ?? 0;

        $lastPKRecord = $this->getLastRankRecord($start, RankType::PK_KING);
        $lastWinCount = $lastPKRecord->win_count ?? 0;
        $lastLoseCount = $lastPKRecord ? $lastPKRecord->round_count - $lastPKRecord->win_count : 0;
        $lastRounds = $lastPKRecord->round_count ?? 0;

        $timeline = carbon($start);
        $endOfWeek = carbon(today())->endOfWeek();
        while($timeline->lte($endOfWeek)) {
            $rankPKRecord = $rankRecords
                ->where('record_date','<=', carbon($timeline)->endOfWeek()->toDateString())
                ->where('rank_type', RankType::PK_KING)
                ->last();
            $rankChampionRecord = $rankRecords
                ->where('record_date', '<=', carbon($timeline)->endOfWeek()->toDateString())
                ->where('rank_type', RankType::CHAMPION)
                ->last();
            $sumRounds = $rankPKRecord ? $rankPKRecord->round_count - $lastRounds : 0;

            $sumWinCount = $rankPKRecord ? $rankPKRecord->win_count - $lastWinCount : 0;
            $sumLoseCount = $rankPKRecord ? $rankPKRecord->round_count - $rankPKRecord->win_count - $lastLoseCount : 0;
            $championCount = $rankChampionRecord ? $rankChampionRecord->win_count - $lastChampionCount : 0;
            $gameCompleteCount = $rankChampionRecord ? $rankChampionRecord->round_count - $lastGameCompleteCount : 0;
            $winRate = $sumRounds > 0 ? $sumWinCount / $sumRounds * 100 : 0;
            $championRate = $gameCompleteCount > 0 ? $championCount / $gameCompleteCount * 100 : 0;
            RankReportHistory::updateOrCreate([
                'element_id' => $this->report->element_id,
                'post_id' => $this->report->post_id,
                'rank_report_id' => $this->report->id,
                'time_range' => RankReportTimeRange::WEEK,
                'start_date' => $timeline->toDateString(),
            ], [
                'rank' => 0, // we mark the rank as 0, then update the rank later
                'win_rate' => $winRate,
                'win_count' => $sumWinCount,
                'lose_count' => $sumLoseCount,
                'champion_count' => $championCount,
                'game_complete_count' => $gameCompleteCount,
                'champion_rate' => $championRate
            ]);


            $lastChampionCount = $rankChampionRecord ? $rankChampionRecord->win_count : 0;
            $lastGameCompleteCount = $rankChampionRecord ? $rankChampionRecord->round_count : 0;
            $lastWinCount = $rankPKRecord ? $rankPKRecord->win_count : 0;
            $lastLoseCount = $rankPKRecord ? $rankPKRecord->round_count - $rankPKRecord->win_count : 0;
            $lastRounds = $rankPKRecord ? $rankPKRecord->round_count : 0;

            $timeline->addWeek();
        }
    }

    protected function buildMonth()
    {

    }

    protected function buildYear()
    {

    }

    protected function getLastStartDate(RankReportTimeRange $range)
    {
        $lastReport = RankReportHistory::where('rank_report_id', $this->report->id)
            ->where('time_range', $range)
            ->orderByDesc('start_date')
            ->first();
        if($lastReport) {
            $start = $lastReport->start_date;
        } else {
            $start = carbon($this->report->post->created_at)->toDateString();
        }

        return $start;
    }

    protected function deleteHistory(RankReportTimeRange $range)
    {
        RankReportHistory::where('rank_report_id', $this->report->id)
            ->where('time_range', $range)
            ->delete();
    }

    protected function getRankRecords($startDate)
    {
        $gameRecords = Rank::where('post_id', $this->report->post_id)
            ->where('element_id', $this->report->element_id)
            ->where('record_date', '>=', $startDate)
            ->orderBy('record_date')
            ->get();

        return $gameRecords;
    }

    protected function getLastRankRecord($beforeDate, $rankType = null)
    {
        $lastRecord = Rank::where('post_id', $this->report->post_id)
            ->where('element_id', $this->report->element_id)
            ->when($rankType, function($query) use ($rankType){
                $query->where('rank_type', $rankType);
            })
            ->where('record_date', '<', $beforeDate)
            ->orderByDesc('record_date')
            ->first();

        return $lastRecord;
    }

}
