<?php


namespace App\Services;
use App\Jobs\ResizeElementImage;
use App\Models\Element;

class ImageThumbnailService
{
    public function makeThumbnail(Element $element, int $maxWidth, int $maxHeight): void
    {
        $url = $element->thumb_url;
        try{
            $image = \Image::make($url);
        }catch (\Exception $e){
            \Log::error('Error making thumbnail', ['element_id' => $element->id, 'url' => $url, 'error' => $e->getMessage()]);
            return;
        }
        $originalWidth = $image->width();
        $originalHeight = $image->height();
        $ratio = $originalWidth / $originalHeight;
        $maxRatio = $maxWidth / $maxHeight;

        if ($ratio > $maxRatio) {
            $maxHeight = round($maxWidth / $ratio);
        } else {
            $maxWidth = round($maxHeight * $ratio);
        }

        ResizeElementImage::dispatch($element, $maxWidth, $maxHeight);
    }
}
