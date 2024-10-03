<?php


namespace App\ScheduleExecutor;

use App\Enums\ElementIssueType;
use App\Enums\PostAccessPolicy;
use App\Models\Element;
use App\Models\ElementIssue;
use App\Models\Post;
use App\Services\ImgurService;

class ImgurScheduleExecutor
{
    public function createAlbum($limit = 50)
    {
        $service = new ImgurService();

        Post::with('imgur_album')
            ->whereHas('imgur_album', function ($query) {
                $query->whereNull('album_id');
            })
            ->whereHas('post_policy', function ($query) {
                $query->where('access_policy', PostAccessPolicy::PUBLIC);
            })
            ->limit($limit)
            ->get()
            ->each(function (Post $post) use ($service) {
                logger('Create album', ['post_id' => $post->id, 'title' => $post->title, 'description' => $post->description]);
                $album = $service->createAlbum($post->title, $post->description);

                if (!$post->imgur_album) {
                    $post->imgur_album()->create([
                        'title' => $post->title,
                        'description' => $post->description
                    ]);
                }

                if (!$album || !$album['success']) {
                    \Log::error('Failed to create album', ['post_id' => $post->id, 'album' => $album]);
                    return false;
                }

                $post->imgur_album->update([
                    'album_id' => $album['data']['id'],
                    'delete_hash' => $album['data']['deletehash'],
                ]);

                // delay 5 second
                sleep(5);
            });
    }

    public function createImage($limit = 50)
    {
        $service = new ImgurService();

        Element::with('posts')
            ->whereHas('posts', function($query){
                $query->whereHas('imgur_album', function($query){
                    $query->whereNotNull('album_id');
                })->whereHas('post_policy', function ($query) {
                    $query->where('access_policy', PostAccessPolicy::PUBLIC);
                });
            })
            ->whereDoesntHave('imgur_image')
            ->where('type', 'image')
            ->whereNotNull('thumb_url')
            ->limit($limit)
            ->orderBy('id', 'desc')
            ->get()
            ->each(function (Element $element) use ($service) {
                logger('Create image', ['element_id' => $element->id, 'url' => $element->thumb_url]);

                foreach($element->posts as $post){
                    $album = $post->imgur_album;
                    if(!$album || $album->album_id == null){
                        continue;
                    }
                    $image = $service->uploadImage($element->thumb_url, $element->title, null, $album->album_id);

                    if (!$image) {
                        \Log::error('Failed to upload image', ['element_id' => $element->id]);
                        return false;
                    }

                    if (!isset($image['success']) || !$image['success']) {
                        if($this->handle400NoSupportType($image)){
                            $element->imgur_image()->create([
                                'imgur_album_id' => $post->imgur_album->id,
                                'title' => $element->title,
                            ]);
                            return;
                        } else {
                            \Log::error('Failed to upload image', ['image' => $image, 'element_id' => $element->id]);
                            return false;
                        }
                    }

                    $element->imgur_image()->create([
                        'image_id' => $image['data']['id'],
                        'imgur_album_id' => $post->imgur_album->id,
                        'title' => $image['data']['title'],
                        'description' => $image['data']['description'],
                        'delete_hash' => $image['data']['deletehash'],
                        'link' => $image['data']['link'],
                    ]);

                    // delay 5 second
                    sleep(5);
                }
            });
    }

    public function updateRemovedImage($limit = 50)
    {
        ElementIssue::with('element.imgur_image')
            ->where('type', ElementIssueType::IMGUR_IMAGE_REMOVED)
            ->whereNull('resolved_at')
            ->whereRelation('element', 'deleted_at', null)
            ->limit($limit)
            ->get()
            ->each(function (ElementIssue $issue) {
                $element = $issue->element;
                $imgurImage = $element->imgur_image;

                if (!$imgurImage) {
                    $issue->update(['resolved_at' => now()]);
                    return;
                }

                $imgurImage->update([
                    'link' => null,
                ]);
                $issue->update(['resolved_at' => now()]);
            });

    }

    protected function handle400NoSupportType($res)
    {
        if(isset($res['status']) && $res['status'] == 400){
            \Log::error('Imgur not support this type', ['res' => $res]);;
            return true;
        }

        if(isset($res['status']) && $res['status'] == 415){
            \Log::error('Invalid type', ['res' => $res]);;
            return true;
        }

        return false;
    }
}
