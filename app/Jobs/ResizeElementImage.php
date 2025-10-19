<?php

namespace App\Jobs;

use App\Models\Element;
use App\Services\Traits\FileHelper;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class ResizeElementImage implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;
    use FileHelper;

    protected Element $element;
    protected $width;
    protected $height;

    protected $column;

    protected $pathPrefix;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Element $element, $width, $height, $column, $pathPrefix)
    {
        $this->element = $element;
        $this->width = $width;
        $this->height = $height;
        $this->column = $column;
        $this->pathPrefix = $pathPrefix;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        // skip if the image is already resized
        if ($this->element->{$this->column} && $this->element->{$this->column} !== $this->element->thumb_url) {
            return;
        }
        // Download the image from the URL to a temporary file
        $tempFilePath = tempnam(sys_get_temp_dir(), 'image_');
        file_put_contents($tempFilePath, file_get_contents($this->element->thumb_url));

        try {
            // Check image format before processing
            $imageInfo = @getimagesize($tempFilePath);
            if ($imageInfo === false) {
                \Log::warning("Cannot get image info for element {$this->element->id}");
                unlink($tempFilePath);
                return;
            }

            $mimeType = $imageInfo['mime'];
            
            // Handle AVIF and other unsupported formats with GD library
            if ($mimeType === 'image/avif' || !$this->isImagickSupported($tempFilePath)) {
                \Log::info("Using GD library for unsupported format: {$mimeType} for element {$this->element->id}");
                $this->handleWithGD($tempFilePath, $mimeType);
                unlink($tempFilePath);
                return;
            }

            $image = new \Imagick($tempFilePath);

            if ($image->getImageFormat() === 'GIF') {
                // Handle GIF resizing
                $image = $this->convertGifToWebp($image);
            } else {
                // Handle other image formats
                $image->setImageFormat('webp'); // Convert to WEBP format
                $image->setImageCompressionQuality(80);
                $image->resizeImage($this->width, $this->height, \Imagick::FILTER_LANCZOS, 1);
            }

            $extension = $this->getExtension($image);
            $path = storage_path('app/tmp/' . $this->generateFileName() . $extension);
            $image->writeImage($path);
            $mineType = $image->getImageMimeType();
            $file = new \Illuminate\Http\UploadedFile($path, 'image_low_thumbnail' . $extension, $mineType, null, true);
            $newPath = $this->moveUploadedFile($file, "{$this->pathPrefix}/{$this->width}x{$this->height}");
            $url = \Storage::url($newPath);
            $this->element->update([
                $this->column => $url
            ]);

            // Delete temp file
            unlink($path);

        } catch (\Exception $e) {
            // Always delete the temporary file
            if (file_exists($tempFilePath)) {
                unlink($tempFilePath);
            }
            throw $e;
        } finally {
            // Always delete the temporary file
            if (file_exists($tempFilePath)) {
                unlink($tempFilePath);
            }
        }
    }

    protected function convertGifToWebp(\Imagick $image)
    {
        // Coalesce the GIF to ensure all frames are available
        $image = $image->coalesceImages();

        // Extract the first frame
        $firstFrame = $image->getImage();

        // Set the format to WEBP
        $firstFrame->setImageFormat('webp');

        // Resize the image
        $firstFrame->resizeImage($this->width, $this->height, \Imagick::FILTER_LANCZOS, 1);

        return $firstFrame;
    }

    protected function getExtension(\Imagick $image)
    {
        try {
            if ($image->getImageFormat() === 'GIF') {
                return ".gif";
            }

            $mime = $image->getImageMimeType();
            if ($mime) {
                $extension = explode('/', $mime)[1];
                return "." . $extension;
            }
        } catch (\Exception $e) {
            logger($e->getMessage());
        }

        return "";
    }

    protected function isImagickSupported($filePath)
    {
        try {
            $test = new \Imagick($filePath);
            $test->destroy();
            return true;
        } catch (\Exception $e) {
            return false;
        }
    }

    protected function handleWithGD($tempFilePath, $mimeType)
    {
        try {
            // Create image resource from file
            $image = null;
            switch ($mimeType) {
                case 'image/jpeg':
                    $image = @imagecreatefromjpeg($tempFilePath);
                    break;
                case 'image/png':
                    $image = @imagecreatefrompng($tempFilePath);
                    break;
                case 'image/gif':
                    $image = @imagecreatefromgif($tempFilePath);
                    break;
                case 'image/webp':
                    $image = @imagecreatefromwebp($tempFilePath);
                    break;
                case 'image/avif':
                    // AVIF support requires PHP 8.1+ with GD extension compiled with AVIF support
                    if (function_exists('imagecreatefromavif')) {
                        $image = @imagecreatefromavif($tempFilePath);
                    }
                    break;
                default:
                    $image = @imagecreatefromstring(file_get_contents($tempFilePath));
                    break;
            }

            if ($image === false) {
                \Log::warning("GD cannot process image for element {$this->element->id}, mime: {$mimeType}");
                return;
            }

            // Get original dimensions
            $originalWidth = imagesx($image);
            $originalHeight = imagesy($image);

            // Calculate new dimensions maintaining aspect ratio
            $ratio = min($this->width / $originalWidth, $this->height / $originalHeight);
            $newWidth = (int)($originalWidth * $ratio);
            $newHeight = (int)($originalHeight * $ratio);

            // Create new image with desired dimensions
            $newImage = imagecreatetruecolor($newWidth, $newHeight);

            // Preserve transparency
            imagealphablending($newImage, false);
            imagesavealpha($newImage, true);
            $transparent = imagecolorallocatealpha($newImage, 0, 0, 0, 127);
            imagefill($newImage, 0, 0, $transparent);

            // Resize
            imagecopyresampled($newImage, $image, 0, 0, 0, 0, $newWidth, $newHeight, $originalWidth, $originalHeight);

            // Save as WebP
            $path = storage_path('app/tmp/' . $this->generateFileName() . '.webp');
            imagewebp($newImage, $path, 80);

            imagedestroy($image);
            imagedestroy($newImage);

            // Upload the file
            $file = new \Illuminate\Http\UploadedFile($path, 'image_thumbnail.webp', 'image/webp', null, true);
            $newPath = $this->moveUploadedFile($file, "{$this->pathPrefix}/{$this->width}x{$this->height}");
            $url = \Storage::url($newPath);
            
            $this->element->update([
                $this->column => $url
            ]);

            // Delete temp file
            unlink($path);

        } catch (\Exception $e) {
            \Log::error("GD processing failed for element {$this->element->id}: " . $e->getMessage());
            throw $e;
        }
    }
}
