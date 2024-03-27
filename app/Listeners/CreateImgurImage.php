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
    public $backoff = 1200;

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
        if(!config('services.imgur.enabled')){
            return;
        }

        if(app()->isLocal()){
            return;
        }
        
        $element = $event->getElement();
        if(!$element){
            \Log::info('Element have been deleted');
            return;
        }

        $post = $event->getPost()->fresh();
        if(!$post || $post->deleted_at){
            \Log::info('Post have been deleted');
            return;
        }

        //todo handle element already have imgur image but we need to reupload
        if($element->imgur_image){
            \Log::info('Element already have imgur image', ['element_id' => $element->id, 'post_id' => $post->id]);
            return;
        }

        \Log::info('[CreateImgurImage] listener handle', ['element_id' => $element->id, 'post_id' => $post->id]);
        
        if(!$post->imgur_album){
            throw new \Exception("Post has no imgur album");
        }

        $res = $this->imgurService->uploadImage(
            $element->source_url,
            $element->title,
            $element->description,
            $post->imgur_album->album_id
        );

        if (!isset($res['success']) || !$res['success']) {
            if($this->handle400NoSupportType($res)){
                return;
            } else {
                \Log::error('Failed to upload image', ['res' => $res]);
                throw new \Exception('Failed to upload image');
            }
        }

        $element->imgur_image()->create([
            'image_id' => $res['data']['id'],
            'imgur_album_id' => $post->imgur_album->id,
            'title' => $res['data']['title'],
            'description' => $res['data']['description'],
            'delete_hash' => $res['data']['deletehash'],
            'link' => $res['data']['link'],
        ]);
    }

    public function handle400NoSupportType($res)
    {
        if(isset($res['status']) && $res['status'] == 400){
            \Log::error('Imgur not support this type', ['res' => $res]);;
            return true;
        }

        return false;
    }
}