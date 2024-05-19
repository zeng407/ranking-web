<?php

namespace App\Jobs;

use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Services\Traits\FileHelper;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldBeUnique;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class MakeVideoThumbnail
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;
    use FileHelper;

    protected $elementId;

    /**
     * Create a new job instance.
     *
     * @return void
     */
    public function __construct($elementId)
    {
        $this->elementId = $elementId;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        $elements = Element::where('type', ElementType::VIDEO)
            ->where('video_source', VideoSource::URL)
            ->where('id', $this->elementId)
            ->get();

        $ffmpeg = \FFMpeg\FFMpeg::create();
        foreach ($elements as $element) {
            if($this->isVideoType($element->thumb_url)){
                $openfile = $ffmpeg->open($element->thumb_url);
            }else{
                $openfile = $ffmpeg->open($element->source_url);
            }
            $tempFile = storage_path('app/tmp/'.$this->generateFileName() . '.jpg');
            $openfile->frame(\FFMpeg\Coordinate\TimeCode::fromSeconds(0.1))
                ->save($tempFile);
            $file = new \Illuminate\Http\UploadedFile($tempFile, 'video_thumbnail.jpg', 'image/jpeg', null, true);
            $newPath = $this->moveUploadedFile($file, 'video-thumbnails');
            $url = \Storage::url($newPath);
            $element->update([
                'thumb_url' => $url
            ]);
            // delete temp file
            unlink($tempFile);   
        }
    }

    protected function isVideoType($url)
    {
        $videoTypes = ['mp4', 'webm', 'ogg'];
        $ext = pathinfo($url, PATHINFO_EXTENSION);
        return in_array($ext, $videoTypes);
    }
}
