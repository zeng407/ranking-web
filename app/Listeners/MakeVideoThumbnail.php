<?php

namespace App\Listeners;

use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Events\VideoElementCreated;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;

class MakeVideoThumbnail implements ShouldQueue
{
    use InteractsWithQueue;

    /**
     * Create the event listener.
     *
     * @return void
     */
    public function __construct()
    {
        //
    }

    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(VideoElementCreated $event)
    {
        if($event->element->type == ElementType::VIDEO && $event->element->video_source == VideoSource::URL){
            $elementId = $event->element->id;
            dispatch(new \App\Jobs\MakeVideoThumbnail($elementId));
        }
    }
}
