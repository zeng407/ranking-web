<?php


namespace App\Enums;


enum TrendTimeRange: string
{
    const TODAY = 'today';
    const WEEK = 'week';
    const MONTH = 'month';
    const YEAR = 'year';
    const ALL = 'all';
}
