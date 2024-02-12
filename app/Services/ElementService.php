<?php


namespace App\Services;


use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Repositories\ElementRepository;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Storage;
use App\Events\ImageElementCreated;
use App\Events\ElementDeleted;

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

        if (isset($paginationOptions['per_page'])) {
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

    public function massStore(string $sourceUrl, string $path, Post $post, $params = []): ?Element
    {
        \Log::debug("guess {$sourceUrl} ...");
        $guess = new ElementSourceGuess($sourceUrl);
        if ($guess->isImage) {
            \Log::debug("got Image");
            return $this->storeImage($sourceUrl, $path, $post, $params);
        }

        if ($guess->isVideo) {
            \Log::debug("got Video");
            return $this->storeVideo($sourceUrl, $path, $post, $params);
        }

        if ($guess->isYoutube) {
            \Log::debug("got Youtube");
            return $this->storeYoutubeVideo($sourceUrl, $post, $params);
        }

        if ($guess->isGFY) {
            \Log::debug("got GFY");
            return $this->storeGfycat($sourceUrl, $post, $params);
        }

        return null;
    }

    public function storePublicImage(UploadedFile $file, string $path, Post $post)
    {
        $saveDir = $path;
        $path = $file->store($saveDir);
        Storage::setVisibility($path, 'public');

        $url = Storage::url($path);
        $thumb = $url;
        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, config('setting.element_title_size'))
        ]);
        
        event(new ImageElementCreated($element, $post));

        return $element;
    }

    public function storeImage(string $sourceUrl, string $path, Post $post, $params = []): ?Element
    {
        try {
            $content = $this->getContent($sourceUrl);
            $fileInfo = pathinfo($sourceUrl);
            $basename = $fileInfo['basename'] . '_' . random_str(8);

            $path = rtrim($path, '/') . '/' . $basename;
            $isSuccess = Storage::put($path, $content, 'public');

            if (!$isSuccess) {
                return null;
            }
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        $title = $params['title'] ?? $fileInfo['filename'];

        $element = $post->elements()->updateOrCreate([
            'source_url' => $sourceUrl,
        ], [
            'path' => $path,
            'source_url' => $sourceUrl,
            'thumb_url' => $sourceUrl,
            'type' => ElementType::IMAGE,
            'title' => $title
        ]);
        event(new ImageElementCreated($element, $post));

        return $element;
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

    public function storeVideo(string $sourceUrl, string $path, Post $post, $params = []): ?Element
    {
        try {
            $fileInfo = pathinfo($sourceUrl);
            $basename = $fileInfo['basename'] . '_' . random_str(8);
            $content = file_get_contents($sourceUrl);

            $path = rtrim($path, '/') . '/' . $basename;
            $isSuccess = Storage::put($path, $content, 'public');

            if (!$isSuccess) {
                return null;
            }
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

    
        $title = $params['title'] ?? $fileInfo['filename'];

        $element = $post->elements()->updateOrCreate([
            'source_url' => $sourceUrl,
        ], [
            'path' => $path,
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
                'source_url' => $info->gfyItem->mp4Url,
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
        $video = app(YoutubeService::class)->query($sourceUrl);
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

            $hourPart = isset($parts[1]) ? (int)$parts[1] : 0;
            $minutePart = isset($parts[2]) ? (int)$parts[2] : 0;
            $secondPart = isset($parts[3]) ? (int)$parts[3] : 0;
            $second = $hourPart * 3600 + $minutePart * 60 + $secondPart;

            $title = $params['title'] ?? $title;
            $element = $post->elements()->updateOrCreate([
                'source_url' => $sourceUrl,
            ],
                [
                    'source_url' => $sourceUrl,
                    'thumb_url' => $thumbUrl,
                    'title' => $title,
                    'type' => ElementType::VIDEO,
                    'video_source' => VideoSource::YOUTUBE,
                    'video_id' => $id,
                    'video_duration_second' => $second,
                ] + $params);

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
            };
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
            \Log::debug($headers);
            return isset($headers['Content-Type'])
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
                if (app(YoutubeService::class)->parseVideoId($url)) {
                    return VideoSource::YOUTUBE;
                }
            } catch (\Exception $exception) {

            }

            //youtube
            try {
                //gfycat.com
                $schemas = parse_url($source);
                $domain = $schemas['host'];
                $regex = '/(^|[^\.]+\.)gfycat\.com$/';
                if (preg_match($regex, $domain) === 1) {

                };
            } catch (\Exception $exception) {

            }
            return false;

            return VideoSource::URL;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }

}
