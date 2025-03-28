<?php


namespace App\Services;
use App\Jobs\ResizeElementImage;
use App\Models\Element;

class ImageThumbnailService
{
    public function makeThumbnail(Element $element, int $maxWidth, int $maxHeight, string $column, string $pathPrefix): void
    {
        $url = $element->thumb_url;
        try{
            $image = new \Imagick($url);
        }catch (\Exception $e){
            \Log::error('Error making thumbnail', ['element_id' => $element->id, 'url' => $url, 'error' => $e->getMessage()]);
            return;
        }

        // Get the page size of the first frame
        $pageInfo = $image->getImagePage();

        // Extract width and height from the page info
        $originalWidth = $pageInfo['width'];
        $originalHeight = $pageInfo['height'];
        $ratio = $originalWidth / $originalHeight;
        $maxRatio = $maxWidth / $maxHeight;

        if ($ratio > $maxRatio) {
            $maxHeight = round($maxWidth / $ratio);
        } else {
            $maxWidth = round($maxHeight * $ratio);
        }

        ResizeElementImage::dispatch($element, $maxWidth, $maxHeight, $column, $pathPrefix);
    }
}
