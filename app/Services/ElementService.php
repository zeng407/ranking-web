<?php


namespace App\Services;


use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\User;
use App\Repositories\ElementRepository;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;

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

    public function storePublic(UploadedFile $file, string $path, Post $post)
    {
        $saveDir = $path;
        $path = $file->store($saveDir);
        Storage::setVisibility($path, 'public');

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, config('post.title_size'))
        ]);

        return $element;
    }

    public function storePublicFromImageUrl(string $sourceUrl, string $path, Post $post)
    {
        try {
            //try check image validation
            if (!@getimagesize($sourceUrl)) {
                return null;
            };

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

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => $fileInfo['filename']
        ]);

        return $element;
    }

    public function storePublicFromVideoUrl(string $sourceUrl, string $path, Post $post)
    {
        try {
            //try check image validation
            if (!$this->isVideoUrl($sourceUrl)) {
                \Log::debug("not video url");
                return null;
            }

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

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $sourceUrl,
            'thumb_url' => $thumb,
            'type' => ElementType::VIDEO,
            'title' => $fileInfo['filename'],
            'video_source' => $this->guestVideoSource($sourceUrl)
        ]);

        return $element;
    }

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

    protected function guestVideoSource(string $url)
    {
        try {
            $schemas = parse_url($url);
            $domain = $schemas['host'];

            //gfycat.com
            $regex = '/(^|[^\.]+\.)gfycat\.com$/';
            if(preg_match($regex, $domain) === 1){
                return VideoSource::GFYCAT;
            }

            return VideoSource::URL;
        }catch (\Exception $exception){
            report($exception);
            return null;
        }
    }

}
