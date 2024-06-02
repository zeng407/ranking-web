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

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct(Element $element, $width, $height)
    {
        $this->element = $element;
        $this->width = $width;
        $this->height = $height;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        $url = $this->element->thumb_url;
        $image = \Image::make($url);
        $image->resize($this->width, $this->height);
        $extension = $this->getExtension($image);
        $path = storage_path('app/tmp/' . $this->generateFileName() . $extension);
        $image->save($path);
        $mineType = $image->mime();
        $file = new \Illuminate\Http\UploadedFile($path, 'image_low_thumbnail'. $extension, $mineType, null, true);
        $newPath = $this->moveUploadedFile($file,  "low/{$this->width}x{$this->height}");
        $url = \Storage::url($newPath);
        $this->element->update([
            'lowthumb_url' => $url
        ]);
        // delete temp file
        unlink($path);
    }

    protected function getExtension(\Intervention\Image\Image $image)
    {
        try{
            $mime = $image->mime();
            if($mime){
                $extension = explode('/', $mime)[1];
                return ".".$extension;
            }
        }catch (\Exception $e){
            logger($e->getMessage());
        }

        return "";
    }
}
