<?php


namespace App\Enums;


enum RankReportTimeRange: string
{
    case WEEK = 'week';
    case MONTH = 'month';
    case YEAR = 'year';
    case ALL = 'all';

    static function toArray(): array
    {
        return [
            RankReportTimeRange::WEEK->value,
            RankReportTimeRange::MONTH->value,
            RankReportTimeRange::YEAR->value,
            RankReportTimeRange::ALL->value,
        ];
    }
}
