<?php


namespace App\Enums;


enum RankReportTimeRange: string
{
    case WEEK = 'week';
    case MONTH = 'month';
    case YEAR = 'year';
    case ALL = 'all';
}
