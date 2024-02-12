<?php

namespace App\Listeners;

use App\Events\ElementDeleted;
use App\Services\ImgurService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Bus\Queueable;

class DeleteImgurImage implements ShouldQueue
{
    use InteractsWithQueue, Queueable;

    protected ImgurService $imgurService;
    
    /**
     * Create the event listener.
     *
     * @return void
     */
    public function __construct(ImgurService $imgurService)
    {
        $this->imgurService = $imgurService;
        $this->onQueue('imgur');
    }


    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(ElementDeleted $event)
    {
        if(!$event->getElement()->imgur_image) {
            return;
        }
        
        logger('[DeleteImgurImage] listener handle', ['element_id' => $event->getElement()->id]);
        $this->imgurService->deleteImage($event->getElement()->imgur_image->image_id);
    }
}