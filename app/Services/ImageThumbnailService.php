<?php


namespace App\Services;
use App\Jobs\ResizeElementImage;
use App\Models\Element;

class ImageThumbnailService
{
    public function makeThumbnail(Element $element, int $maxWidth, int $maxHeight, string $column, string $pathPrefix): void
    {
        $url = $element->thumb_url;
        
        // Check if the URL returns 0 byte
        try {
            $response = \Illuminate\Support\Facades\Http::timeout(10)->head($url);
            if ($response->successful()) {
                $contentLength = (int)($response->header('Content-Length') ?? 0);
                if ($contentLength === 0) {
                    \Log::warning('Thumbnail URL is 0 byte, updating to source_url', [
                        'element_id' => $element->id,
                        'url' => $url,
                        'column' => $column,
                        'source_url' => $element->source_url
                    ]);
                    
                    // Update the column to source_url
                    if (!empty($element->source_url)) {
                        $element->update([$column => $element->source_url]);
                    }
                    return;
                }
            }
        } catch (\Exception $e) {
            // If HEAD request fails, continue to try Imagick
        }
        
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
