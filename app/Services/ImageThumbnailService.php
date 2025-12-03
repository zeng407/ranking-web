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

        // Validate URL: allow only http/https, block local IPs/hostnames
        if (!filter_var($url, FILTER_VALIDATE_URL) || !preg_match('/^https?:\/\//i', $url)) {
            \Log::error('Invalid thumbnail URL', ['element_id' => $element->id, 'url' => $url]);
            return;
        }
        // Optionally, add more SSRF protections here (e.g., block private IPs)
        $tempFile = tempnam(sys_get_temp_dir(), 'thumb_');
        try {
            $response = \Illuminate\Support\Facades\Http::timeout(10)->get($url);
            if (!$response->successful() || !$response->body()) {
                throw new \Exception('Failed to download image');
            }
            file_put_contents($tempFile, $response->body());
            $image = new \Imagick($tempFile);
        } catch (\Exception $e) {
            \Log::error('Error making thumbnail', ['element_id' => $element->id, 'url' => $url, 'error' => $e->getMessage()]);

            if (!empty($element->source_url) && $element->{$column} !== $element->source_url) {
                $element->update([$column => $element->source_url]);
            }

            if (isset($tempFile) && file_exists($tempFile)) {
                @unlink($tempFile);
            }
            return;
        }

        // Clean up temp file after use
        if (isset($tempFile) && file_exists($tempFile)) {
            @unlink($tempFile);
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
