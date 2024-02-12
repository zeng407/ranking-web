<?php

namespace App\Listeners;

use App\Events\PostCreated;
use App\Services\ImgurService;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Queue\InteractsWithQueue;
use Exception;
use Illuminate\Bus\Queueable;

class CreateImgurAlbum implements ShouldQueue
{
    use InteractsWithQueue, Queueable;

    protected $imgurService;

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
        logger('CreateImgurAlbum listener created');
        $this->imgurService = $imgurService;
        $this->onQueue('imgur');
    }

    /**
     * Handle the event.
     *
     * @param  object  $event
     * @return void
     */
    public function handle(PostCreated $event)
    {
        $post = $event->getPost();
        if(!$post){
            logger('Post have been deleted');
            return;
        }
        logger('CreateImgurAlbum listener handle', ['post_id' => $post->id]);

        $data = $this->imgurService->createAlubm($post->title, $post->description);
        // sample data
        //{
        //  "data": {
        //      "id": "QFH735d",
        //      "deletehash": "pPEHoApUaxS220Q"
        //  },
        //  "success": true,
        //  "status": 200
        //}

        if (!$data['success']) {
            throw new Exception('Failed to create album');
        }

        if (!$post->imgur_album) {
            throw new Exception('Album not found');
        }

        $post->imgur_album->update([
            'album_id' => $data['data']['id'],
            'delete_hash' => $data['data']['deletehash'],
        ]);
    }
}