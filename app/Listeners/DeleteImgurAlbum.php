<?php

namespace App\Listeners;

use App\Events\PostDeleted;
use App\Services\ImgurService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Bus\Queueable;

class DeleteImgurAlbum implements ShouldQueue
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
    public function handle(PostDeleted $event)
    {
        logger('[DeleteImgurAlbum] listener handle', ['post_id' => $event->getPost()->id]);
        $this->imgurService->deleteAlbum($event->getPost()->imgur_album->album_id);
    }
}
