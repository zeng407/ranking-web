<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use Storage;
use App\Enums\ElementType;
use App\Events\ImageElementCreated;
use App\Services\ImgurService;

class ImgurElementHandler implements InterfaceElementHandler
{
    use FileHelper;


    public function storeArray(string $sourceUrl, string $serial, $params = []): ?array
    {
        try {
            $directory = $serial;
            // if url contains imgur
            if (strpos($sourceUrl, 'imgur.com') !== false) {
                $imgurService = app(ImgurService::class);
                $galleryId = $imgurService->parseGalleryAlbumId($sourceUrl);
                logger("galleryId: {$galleryId}");
                if($galleryId === null) {
                    return null;
                }

                // if url contains gallery
                if (strpos($sourceUrl, '/gallery/') !== false) {
                    $image = $this->getImageFromImgurGallery($galleryId) ?? $this->getImageFromImgur($galleryId);
                } else if (strpos($sourceUrl, '/a/') !== false) {
                    $image = $this->getAlbumImage($galleryId);
                } else if (strpos($sourceUrl, '/t/') !== false) {
                    $image = $this->getAlbumImage($galleryId);
                } else {
                    $image = $this->getImageFromImgur($galleryId);
                }

                if($image === null) {
                    return null;
                }

                $link = $image['link'];
                $storageImage = $this->downloadImage($link, $directory);
            } else {
                logger("not gallery");
                return null;
            }

            $title = $params['title'] ?? $image['title'];
            $thumb = Storage::url($storageImage->getPath());

            $type = $this->guessMediaType($image['type']);
            $videoSource = $type === ElementType::VIDEO ? VideoSource::IMGUR : null;

            return [
                'title' => $title,
                'thumb_url' => $thumb,
                'path' => $storageImage->getPath(),
                'video_source' => $videoSource,
                'source_url' => $sourceUrl,
                'type' => $type,
                'image' => $image,
            ];
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $array = $this->storeArray($sourceUrl, $post->serial, $params);
            if(!$array){
                return null;
            }

            // find old_element and delete files
            if ($params['old_source_url'] ?? null) {
                $oldElement = $post->elements()->where('source_url', $params['old_source_url'])->first();
                if ($oldElement) {
                    $this->deleteElemntFile($oldElement->path);
                    $this->deleteElemntFile($oldElement->thumb_url);
                    $this->deleteElemntFile($oldElement->lowthumb_url);
                    $this->deleteElemntFile($oldElement->mediumthumb_url);
                }
            }

            $element = $post->elements()->updateOrCreate([
                'source_url' => $params['old_source_url'] ?? $sourceUrl,
            ], [
                'path' => $array['path'],
                'video_source' => $array['video_source'],
                'source_url' => $sourceUrl,
                'thumb_url' => $array['thumb_url'],
                'mediumthumb_url' => null,
                'lowthumb_url' => null,
                'type' => $array['type'],
                'title' => $array['title'],
            ]);

            $image = $array['image'];
            if($element->imgur_image) {
                $element->imgur_image->delete();
            }
            $element->imgur_image()->create([
                'image_id' => $image['id'],
                'imgur_album_id' => null,
                'title' => $image['title'],
                'description' => $image['description'],
                'delete_hash' => null,
                'link' => $image['link'],
            ]);

            event(new ImageElementCreated($element, $post));
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return $element;
    }

    protected function getImageFromImgurGallery(string $galleryId): ?array
    {
        try {
            $imgurService = app(ImgurService::class);
            $res = $imgurService->getGalleryAlbumImages($galleryId);
            logger("getImageFromImgurGallery");
            logger($res);
            if(isset($res['success']) && $res['success']
                && isset($res['status']) && $res['status'] === 200
                && isset($res['data']) && isset($res['data']['images'])) {
                $images = $res['data']['images'];
                if(count($images) === 0) {
                    return null;
                }

                $image = $res['data']['images'][0];
                if(isset($image['link'])) {
                    return $image;
                }
            }
            return null;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    protected function getImageFromImgur(string $galleryId): ?array
    {
        try {
            $imgurService = app(ImgurService::class);
            $res = $imgurService->getImage($galleryId);
            logger("getImageFromImgur");
            logger($res);
            if(isset($res['success']) && $res['success']
                && isset($res['status']) && $res['status'] === 200
                && isset($res['data'])) {

                $image = $res['data'];
                if(isset($image['link'])) {
                    return $image;
                }
            }
            return null;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    protected function getAlbumImage(string $albumId)
    {
        try {
            $imgurService = app(ImgurService::class);
            $res = $imgurService->getAlbumImages($albumId);
            logger("getAlbumImage");
            logger($res);
            if(isset($res['success']) && $res['success'] && isset($res['status']) && $res['status'] === 200 && isset($res['data'])) {
                return $res['data'][0];
            }
            return null;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    protected function guessMediaType(string $type)
    {
        // 'viode/mp4
        if(strpos($type, 'video') !== false) {
            return ElementType::VIDEO;
        }else if(strpos($type, 'image') !== false) {
            return ElementType::IMAGE;
        }else {
            \Log::warning("unknown media type: {$type}");
            return ElementType::IMAGE;
        }
    }
}
