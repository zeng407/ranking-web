<?php

namespace App\Listeners;

use App\Events\ImageElementCreated;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use App\Services\ImgurService;
use Illuminate\Bus\Queueable;

class CreateImgurImage implements ShouldQueue
{
    use InteractsWithQueue, Queueable;

    protected ImgurService $imgurService;

    /**
     * The number of times the job may be attempted.
     *
     * @var int
     */
    public $tries = 5;

    /**
     * The number of seconds to wait before retrying the job.
     *
     * @var int
     */
    public $backoff = 30;

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
    public function handle(ImageElementCreated $event)
    {
        $element = $event->getElement();
        if(!$element){
            logger('Element have been deleted');
            return;
        }

        $post = $event->getPost();
        if(!$post){
            logger('Post have been deleted');
            return;
        }
        logger('[CreateImgurImage] listener handle', ['element_id' => $element->id, 'post_id' => $post->id]);
        
        if(!$post->imgur_album){
            throw new \Exception("Post has no imgur album");
        }

        $res = $this->imgurService->uploadImage(
            $element->source_url,
            $element->title,
            $element->description,
            $post->imgur_album->album_id
        );

        if (!$res['success']) {
            logger('Failed to upload image', ['res' => $res]);
            throw new \Exception('Failed to upload image');
        }

        $element->imgur_image()->create([
            'image_id' => $res['data']['id'],
            'imgur_album_id' => $post->imgur_album->id,
            'title' => $res['data']['title'],
            'description' => $res['data']['description'],
            'delete_hash' => $res['data']['deletehash'],
            'link' => $res['data']['link'],
        ]);
        $element->update([
            'thumb_url' => $res['data']['link'],
        ]);
    }
}
