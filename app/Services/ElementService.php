<?php


namespace App\Services;


use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Repositories\ElementRepository;
use App\Services\Models\StoragedImage;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Storage;
use App\Events\ImageElementCreated;
use App\Events\ElementDeleted;
use Ramsey\Uuid\Uuid;

class ElementService
{
    protected $repo;

    public function __construct(ElementRepository $elementRepository)
    {
        $this->repo = $elementRepository;
    }

    public function getLists(array $conditions, array $paginationOptions = [])
    {
        $query = $this->repo->filter($conditions);

        $perPage = 15;

        if (isset ($paginationOptions['per_page'])) {
            $perPage = $paginationOptions['per_page'];
        }

        return $query->paginate($perPage);
    }

    public function getExistsElement(string $sourceUrl, Post $post): ?Element
    {
        return $post->elements()->where(function ($query) use ($sourceUrl) {
            $query->Where('source_url', $sourceUrl);
        })
            ->first();
    }

    public function massStore(string $sourceUrl, string $directory, Post $post, $params = []): ?Element
    {
        $guess = $this->guessSourceType($sourceUrl);
        if ($guess->isImage) {
            logger("got Image");
            return $this->storeImage($sourceUrl, $directory, $post, $params);
        } elseif ($guess->isImgur) {
            logger("got Imgur");
            return $this->storeImgurImage($sourceUrl, $directory, $post, $params);
        } elseif ($guess->isVideo) {
            logger("got Video");
            return $this->storeVideo($sourceUrl, $directory, $post, $params);
        } elseif ($guess->isYoutube) {
            logger("got Youtube");
            return $this->storeYoutubeVideo($sourceUrl, $post, $params);
        } elseif ($guess->isGFY) {
            logger("got GFY");
            return $this->storeGfycat($sourceUrl, $post, $params);
        } else {
            logger("got Unknown");
        }

        return null;
    }

    protected function guessSourceType(string $url)
    {
        logger("guess {$url} ...");
        return new ElementSourceGuess($url);
    }

    public function storeYoutubeEmbed(string $embedCode, Post $post, $params = []): ?Element
    {
        // extract video id from embed code
        preg_match('/src="https:\/\/www.youtube.com\/embed\/([^"]+)"/', $embedCode, $matches);
        $videoUrl = $matches[1] ?? null;
        
        // validate video id 
        // example : 1H2cyhWYXrE?si=btfjgIQDNUoNuriT&amp;clip=UgkxeWL6j9ODyTnJpJe6Ris_NgNzLFls3SyG&amp;clipt=ELidBRjQkgY
        $validate = preg_match('/^[a-zA-Z0-9?&;=_-]+$/', $videoUrl) && strlen($videoUrl) <= 99;
        if (!$validate){
            return null;
        }

        $embedCode = "<iframe width=\"100%\" height=\"270\" src=\"https://www.youtube.com/embed/{$videoUrl}\" title=\"YouTube video player\" " . 
            "frameborder=\"0\" allow=\"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share\" ". 
            "referrerpolicy=\"strict-origin-when-cross-origin\" allowfullscreen></iframe>";
        $videoId = explode('?', $videoUrl)[0];
        $thumbUrl = "https://img.youtube.com/vi/{$videoId}/hqdefault.jpg";
        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'source_url' => $embedCode,
            'thumb_url' => $thumbUrl,
            'type' => ElementType::VIDEO,
            'title' => $params['title'] ?? '',
            'video_source' => VideoSource::YOUTUBE_EMBED,
            'video_id' => $videoId,
        ]);

        return $element;
    }


    public function storePublicImage(UploadedFile $file, string $directory, Post $post)
    {
        $path = $this->moveUploadedFile($file, $directory);

        $url = Storage::url($path);
        $thumb = $url;
        $title = mb_substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, config('setting.element_title_size'));
        $title = preg_replace('/[\n\r\t]/', '', $title);
        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => $title
        ]);

        event(new ImageElementCreated($element, $post));

        return $element;
    }

    public function moveUploadedFile(UploadedFile $file, string $directory): string|bool
    {
        $path = $file->storeAs($directory, $this->generateFileName() . '.' . $file->getClientOriginalExtension());
        Storage::setVisibility($path, 'public');
        return $path;
    }

    public function downloadImage(string $url, string $directory): ?StoragedImage
    {
        $content = $this->getContent($url);
        $fileInfo = pathinfo($url);
        $basename = $this->generateFileName();
        if (isset ($fileInfo['extension'])) {
            $basename .= '.' . $fileInfo['extension'];
        }

        $path = rtrim($directory, '/') . '/' . $basename;
        $isSuccess = Storage::put($path, $content, 'public');
        if (!$isSuccess) {
            return null;
        }
        return new StoragedImage($url, $path, $fileInfo);
    }

    public function storeImage(string $sourceUrl, string $directory, Post $post, $params = []): ?Element
    {
        try {
            $storageImage = $this->downloadImage($sourceUrl, $directory);

            if ($storageImage === null) {
                return null;
            }
            $fileInfo = $storageImage->getFileInfo();
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        $title = $params['title'] ?? $fileInfo['filename'];
        $localUrl = Storage::url($storageImage->getPath());

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ?? $sourceUrl,
        ], [
            'path' => $storageImage->getPath(),
            'source_url' => $sourceUrl,
            'thumb_url' => $localUrl,
            'type' => ElementType::IMAGE,
            'title' => $title
        ]);
        event(new ImageElementCreated($element, $post));

        return $element;
    }

    public function storeImgurImage(string $sourceUrl,string $directory, Post $post, $params = []): ?Element
    {
        try {
            // if url contains imgur
            if (strpos($sourceUrl, 'imgur.com') !== false) {
                $image = $this->getImageFromImgurGallery($sourceUrl) ??
                    $this->getImageFromImgur($sourceUrl);

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
            $localUrl = Storage::url($storageImage->getPath());
            $element = $post->elements()->updateOrCreate([
                'source_url' => $params['old_source_url'] ?? $sourceUrl,
            ], [
                'path' => $storageImage->getPath(),
                'source_url' => $sourceUrl,
                'thumb_url' => $localUrl,
                'type' => ElementType::IMAGE,
                'title' => $title
            ]);

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

    protected function getImageFromImgurGallery(string $sourceUrl): ?array
    {
        try {
            $imgurService = app(ImgurService::class);
            $galleryId = $imgurService->parseGalleryAlbumId($sourceUrl);
            logger("galleryId: {$galleryId}");
            $res = $imgurService->getGalleryAlbumImages($galleryId);
            if(isset($res['success']) && $res['success'] && isset($res['status']) && $res['status'] === 200 && isset($res['data']) && isset($res['data']['images'])) {
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

    protected function getImageFromImgur(string $sourceUrl): ?array
    {
        try {
            $imgurService = app(ImgurService::class);
            $galleryId = $imgurService->parseGalleryAlbumId($sourceUrl);
            $res = $imgurService->getImage($galleryId);
            if(isset($res['success']) && $res['success'] && isset($res['status']) && $res['status'] === 200 && isset($res['data'])) {
                
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

    protected function generateFileName()
    {
        return Uuid::uuid4()->toString();
    }

    protected function getContent(string $sourceUrl)
    {
        $content = null;

        try {
            $ch = curl_init();
            curl_setopt($ch, CURLOPT_URL, $sourceUrl);
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, 1);
            curl_setopt($ch, CURLOPT_USERAGENT, 'Mozilla/5.0 (Windows; U; Windows NT 5.1; en-US; rv:1.8.1.13) Gecko/20080311 Firefox/2.0.0.13');
            $content = curl_exec($ch);
            curl_close($ch);
        } catch (\Exception $exception) {
        }

        try {
            $content = file_get_contents($sourceUrl);
        } catch (\Exception $exception) {
        }

        return $content;
    }

    public function storeVideo(string $sourceUrl, string $directory, Post $post, $params = []): ?Element
    {
        // try {
        //     $fileInfo = pathinfo($sourceUrl);
        // } catch (\Exception $exception) {
        //     report($exception);
        //     return null;
        // }

        //todo make thumb from video

        $title = $params['title'] ?? 'video';

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ?? $sourceUrl,
        ], [
            'path' => null,
            'source_url' => $sourceUrl,
            'thumb_url' => $sourceUrl,
            'type' => ElementType::VIDEO,
            'title' => $title,
            'video_source' => VideoSource::URL
        ]);

        return $element;
    }

    public function storeGfycat(string $sourceUrl, Post $post, $params = []): ?Element
    {
        try {
            $gfycatService = app(GfycatService::class);
            $id = $gfycatService->getId($sourceUrl);
            $info = $gfycatService->getInfo($id);

            $title = $params['title'] ?? $info->gfyItem->title;

            $element = $post->elements()->updateOrCreate([
                'source_url' => $params['old_source_url'] ?? $info->gfyItem->mp4Url,
            ], [
                'source_url' => $info->gfyItem->mp4Url,
                'thumb_url' => $info->gfyItem->posterUrl,
                'type' => ElementType::VIDEO,
                'title' => $title,
                'video_source' => VideoSource::GFYCAT
            ]);

        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return $element;
    }

    public function storeYoutubeVideo($sourceUrl, Post $post, $params = []): ?Element
    {
        $video = $this->getYoutubeService()->query($sourceUrl);
        if (!$video) {
            return null;
        }
        try {
            $thumb = $video->getSnippet()->getThumbnails()->getHigh() ?:
                $video->getSnippet()->getThumbnails()->getMedium() ?:
                $video->getSnippet()->getThumbnails()->getStandard() ?:
                $video->getSnippet()->getThumbnails()->getMaxres() ?:
                $video->getSnippet()->getThumbnails()->getDefault();
            $thumbUrl = $thumb->getUrl();
            $title = $video->getSnippet()->getTitle();
            $id = $video->getId();
            $duration = $video->getContentDetails()->getDuration();
            preg_match('/^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$/', $duration, $parts);

            $hourPart = isset ($parts[1]) ? (int) $parts[1] : 0;
            $minutePart = isset ($parts[2]) ? (int) $parts[2] : 0;
            $secondPart = isset ($parts[3]) ? (int) $parts[3] : 0;
            $second = $hourPart * 3600 + $minutePart * 60 + $secondPart;

            $title = $params['title'] ?? $title;
            $element = $post->elements()->updateOrCreate(
                [
                    'source_url' => $params['old_source_url'] ?? '', // we don't replace old element if it's youtube video
                ],
                [
                    'source_url' => $sourceUrl,
                    'thumb_url' => $thumbUrl,
                    'title' => mb_substr($title, 0, config('setting.element_title_size')),
                    'type' => ElementType::VIDEO,
                    'video_source' => VideoSource::YOUTUBE,
                    'video_id' => $id,
                    'video_duration_second' => $second,
                ] + $params
            );

            return $element;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    public function delete(Element $element)
    {
        $element->posts()->detach();
        $element->delete();

        event(new ElementDeleted($element));
    }

    /**
     * @param $sourceUrl
     * @return bool
     * @deprecated
     */
    protected function isImageUrl($sourceUrl)
    {
        try {
            if (@getimagesize($sourceUrl)) {
                return true;
            }
            ;
        } catch (\Exception $exception) {
        }
        return false;
    }

    /**
     * @param string $url
     * @return bool
     * @deprecated
     */
    protected function isVideoUrl(string $url)
    {
        try {
            //accept header content-type
            //video/mpeg
            //video/mp4
            //video/quicktime
            //video/x-ms-wmv
            //video/x-msvideo
            //video/x-flv
            //video/webm
            //video/*

            $headers = get_headers($url, true);
            logger($headers);
            return isset ($headers['Content-Type'])
                && explode('/', $headers['Content-Type'])[0] === 'video';
        } catch (\Exception $exception) {
            report($exception);
            return false;
        }
    }

    /**
     * @param string $url
     * @return string|null
     * @deprecated
     */
    protected function guestVideoSource(string $url)
    {
        try {
            //youtube
            try {
                if ($this->getYoutubeService()->parseVideoId($url)) {
                    return VideoSource::YOUTUBE;
                }
            } catch (\Exception $exception) {

            }

            return null;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

    protected function getYoutubeService()
    {
        /** @var YoutubeService  */
        $youtubeService = app(YoutubeService::class);
        return $youtubeService;
    }

}
