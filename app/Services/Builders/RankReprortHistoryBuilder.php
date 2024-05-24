<?php

namespace App\Services\Builders;

use App\Enums\RankReportTimeRange;
use App\Models\Element;
use App\Models\Game1V1Round;
use App\Models\Post;
use App\Models\RankReport;
use App\Models\RankReportHistory;

class RankReprortHistoryBuilder
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

        // 錯誤容忍度，如果有遺漏的資料，最多補 3 天
        // fail tolerance, if there are missing data, at most fill in 3 days
        $start = carbon($start)->subDay(3);
        
        $gameRoundRecords = $this->getGameRoundRecords();
        $lastRecord = $gameRoundRecords->last();
        $sumWinCount = $lastRecord ? $lastRecord->win_count : 0;
        $sumLoseCount = $lastRecord ? $lastRecord->lose_count : 0;
        $sumRounds = $sumWinCount + $sumLoseCount;
        $timeline = carbon($start);
        while($timeline->lte(today())) {
            $query = $gameRoundRecords->where('date', carbon($timeline)->toDateString())->first();
            if($query){
                $sumWinCount = $query->win_count;
                $sumLoseCount = $query->lose_count;
                $sumRounds = $query->win_count + $query->lose_count;
            }
            $winRate = $sumRounds > 0 ? $sumWinCount / $sumRounds * 100 : 0;
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
                'lose_count' => $sumLoseCount
            ]);

            $timeline->addDay();
        }
    }

    protected function buildWeek()
    {
        
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
            $start = $this->report->post->created_at;
        }   

        return $start;
    }

    protected function deleteHistory(RankReportTimeRange $range)
    {
        RankReportHistory::where('rank_report_id', $this->report->id)
            ->where('time_range', $range)
            ->delete();
    }

    protected function getGameRoundRecords()
    {
        $gameRecords = Game1V1Round::join('games', 'games.id', '=', 'game_1v1_rounds.game_id')
            ->where('games.post_id', $this->report->post_id)
            ->where(function($query){
                $query->where('game_1v1_rounds.winner_id', $this->report->element_id)
                    ->orWhere('game_1v1_rounds.loser_id', $this->report->element_id);
            })
            ->orderByRaw('date(game_1v1_rounds.created_at)')
            ->selectRaw(
                'date(game_1v1_rounds.created_at) as date,
                sum(game_1v1_rounds.winner_id = ?) as win_count,
                sum(game_1v1_rounds.loser_id = ?) as lose_count',
                [$this->report->element_id, $this->report->element_id])
            ->groupByRaw('date(game_1v1_rounds.created_at)')
            ->get();
        
        // reduce every count to next day
        $gameRecords->reduce(function($carry, $item){
            // skip the first item
            if(!$carry) {
                return $item;
            }
            $item->win_count += $carry->win_count;
            $item->lose_count += $carry->lose_count;
            return $item;
        });

        return $gameRecords;
    }

}