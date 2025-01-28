<?php


namespace App\ScheduleExecutor;

use App\Models\Element;
use App\Services\ImageThumbnailService;
use App\Enums\ElementType;
use Storage;

class ElementScheduleExecutor
{
    public function removeDeletedFiles($limit = 1000)
    {
        $elements = Element::where('type', ElementType::IMAGE)
            ->withTrashed()
            ->whereNotNull('deleted_at')
            ->whereNotNull('path')
            ->limit($limit)
            ->get();


        foreach ($elements as $element) {
            if (Storage::exists($element->path)) {
                \Log::info('Deleting file: ' . $element->path);
                Storage::delete($element->path);
            }

            $thumbPath = str_replace(Storage::url(''), '', $element->thumb_url);
            if (Storage::exists($thumbPath)) {
                \Log::info('Deleting thumb file: ' . $thumbPath);
                Storage::delete($thumbPath);
            }

            $lowThumbPath = str_replace(Storage::url(''), '', $element->lowthumb_url);
            if (Storage::exists($lowThumbPath)) {
                \Log::info('Deleting lowthumb file: ' . $lowThumbPath);
                Storage::delete($lowThumbPath);
            }

            $mediumthumbPath = str_replace(Storage::url(''), '', $element->mediumthumb_url);
            if (Storage::exists($mediumthumbPath)) {
                \Log::info('Deleting mediumthumb file: ' . $mediumthumbPath);
                Storage::delete($mediumthumbPath);
            }

            $element->path = null;
            $element->save();
        }
    }
}
