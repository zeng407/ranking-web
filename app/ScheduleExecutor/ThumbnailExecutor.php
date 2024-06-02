<?php


namespace App\ScheduleExecutor;

use App\Models\Element;
use App\Services\ImageThumbnailService;

class ThumbnailExecutor
{
    public function makeElementThumbnails($limit = 1000)
    {
        $service = new ImageThumbnailService();

        // find elements that need to be resized
        $elements = Element::where('type', 'image')
            ->whereNull('lowthumb_url')
            ->select('id', 'thumb_url')
            ->orderByDesc('id')
            ->limit($limit)
            ->get();

        foreach ($elements as $element) {
            $service->makeThumbnail($element, 400, 400);
        }
    }
}
