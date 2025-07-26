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
}
