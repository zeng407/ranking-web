<?php

namespace App\Http\Controllers\Api;

use App\Enums\ApiResponseCode;
use App\Enums\ElementType;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Models\Element;
use App\Models\Post;
use App\Services\ElementService;
use App\Services\ElementSourceGuess;
use App\Services\Traits\FileHelper;
use App\Services\YoutubeService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class ElementController extends Controller
{
    use FileHelper;

    protected $youtube;

    protected $elementService;

    public function __construct(YoutubeService $service, ElementService $elementService)
    {
        $this->youtube = $service;
        $this->elementService = $elementService;
    }

    public function createMedia(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required|string',
            'file' => [
                'required',
                'mimetypes:image/jpeg,image/png,image/bmp,image/webp,image/gif,video/avi,video/mpeg,video/mp4',
                'max:'.(config('setting.upload_media_file_size_mb') * 1024),
            ],
        ]);

        $post = $this->getPost($request->input('post_serial'));

        // check elements count
        if ($post->elements()->count() >= config('setting.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        $path = $request->input('post_serial');

        //rate limit for uploading
        //30MB per minute or 50 files per minute
        try{
            $this->attemptUploadRateLimit($request);
        }catch(\Exception $e){
            return api_response(ApiResponseCode::UPLOAD_SIZE_RATE_LIMIT, 422);
        }

        $element = $this->elementService->storeUploadedFile($request->file('file'), $path, $post);
        return PostElementResource::make($element);
    }

    public function batchCreate(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required|string',
            'url' => 'required|string',
        ]);
        $urls = $request->input('url');

        // if the url is a youtube video embed and iframe
        try{
            if ($element = $this->tryStroeYoutubeEmbed($request, $urls)) {
                return PostElementResource::collection([$element]);
            }
        }catch(\Exception $e){
            return api_response(ApiResponseCode::INVALID_URL, 422);
        }


        try {
            // url string to array and trim url
            // example: convert "example1.com, www.example.com" to
            // ["example.com", "www.example.com"]
            $urls = array_unique(explode(',', $urls));
            $urls = array_filter(array_map(function ($url) {
                return trim($url);
            }, $urls));

            $urlCount = 0;
            foreach ($urls as $urlStr) {
                // convert string to url + title
                // example: convert "example.com title" to
                // ["example.com", "title"]
                $parts = explode(" ", $urlStr, 2);
                $url = $parts[0] ?? "";
                \Validator::validate([
                    'url' => $url
                ], [
                    'url' => 'url'
                ]);

                $urlCount++;
            }
        } catch (\Exception $exception) {
            report($exception);
            return api_response(ApiResponseCode::INVALID_URL, 422, [
                'error_url' => $urlStr,
            ]);
        }

        if($urlCount > config('setting.upload_url_at_a_time')){
            return api_response(ApiResponseCode::OVER_UPLOAD_LIMIT, 422);
        }

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() + $urlCount > config('setting.post_max_element_count')) {
            logger("{$post->serial} OVER_ELEMENT_SIZE");
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        try {
            $path = $request->post_serial;
            $elements = [];
            $errors = [];
            foreach ($urls as $urlStr) {
                $parts = explode(" ", $urlStr,2);
                $url = $parts[0] ?? "";
                $title = $parts[1] ?? "";

                if(!$this->checkMediaSize($url)){
                    $errors[] = $url;
                    continue;
                }

                $elementParams = [];
                if($title){
                    $elementParams['title'] = mb_substr($title,0,config('setting.element_title_size'));
                }
                $element = $this->elementService->massStore($url, $path, $post, $elementParams);
                logger("massStore return");
                logger($element);
                if (!$element) {
                    $errors[] = $url;
                    continue;
                }
                $elements[] = $element;
            }
        } catch (\Exception $exception) {
            report($exception);
            return api_response(ApiResponseCode::INVALID_URL, 422);
        }

        if ($elements === []) {
            logger("NO_ELEMENT_CREATED");
            return api_response(ApiResponseCode::NO_ELEMENT_CREATED, 422, [
                'error_url' => head($errors),
                'error_urls' => $errors
            ]);
        }

        return PostElementResource::collection($elements);
    }


    public function update(Request $request, Element $element)
    {
        /** @see ElementPolicy::update() */
        $this->authorize('update', $element);

        $data = $request->validate([
            'post_serial' => 'required|string',
            'title' => ['sometimes', 'string', 'max:' . config('setting.element_title_size')],
            'video_start_second' => 'sometimes|integer',
            'video_end_second' => 'sometimes|integer',
            'url' => 'sometimes|string',
            'path_id' => 'sometimes|string',
        ]);

        $post = $this->getPost($request->input('post_serial'));

        if(isset($data['url'])){
            try{
                $newElement = $this->tryStroeYoutubeEmbed($request, $data['url'], ['old_source_url' => $element->source_url]);
                if ($newElement) {
                    return PostElementResource::make($newElement);
                }
            }catch(\Exception $e){
                return api_response(ApiResponseCode::INVALID_URL, 422);
            }

            // This will update the element whose 'source_url' matches 'old_source_url'.
            $element = $this->elementService->massStore(
                $data['url'],
                $post->serial,
                $post,
                [
                    'old_source_url' => $element->source_url,
                    'title' => $element->title
                ]
            );
            if(!$element){
                return api_response(ApiResponseCode::INVALID_URL, 422);
            }
        } elseif (isset($data['path_id'])){
            $fileInfo = $this->getFileInfoCache($data['path_id']);
            if($fileInfo == null){
                return api_response(ApiResponseCode::INVALID_PATH, 422);
            }

            if(isset($fileInfo['path']) && \Storage::exists($fileInfo['path'])){
                $url = \Storage::url($fileInfo['path']);
                $isImage = $fileInfo['is_image'] ?? false;
                $data['thumb_url'] = $url;
                $data['mediumthumb_url'] = null;
                $data['lowthumb_url'] = null;
                $data['source_url'] = $url;
                $data['type'] = !$isImage ? ElementType::VIDEO: ElementType::IMAGE;
                $data['video_source'] = null;
                $data['video_id'] = null;
                $data['video_duration_second'] = null;
                $data['video_start_second'] = null;
                $data['video_end_second'] = null;

                // This will update the element whose 'source_url' matches 'old_source_url'.
                $element = $this->elementService->massStore(
                    $url,
                    $post->serial,
                    $post,
                    [
                        'old_source_url' => $element->source_url,
                        'title' => $element->title
                    ]
                );
                if(!$element){
                    return api_response(ApiResponseCode::INVALID_URL, 422);
                }
            }
        }

        unset($data['url']);
        unset($data['path_id']);

        $element->update($data);

        return PostElementResource::make($element);
    }

    public function upload(Request $request, Element $element)
    {
        /** @see ElementPolicy::update() */
        $this->authorize('update', $element);

        $request->validate([
            'post_serial' => 'required|string',
            'file' => [
                'required',
                'mimetypes:image/jpeg,image/png,image/bmp,image/webp,image/gif,video/avi,video/mpeg,video/mp4',
                'max:'.(config('setting.upload_media_file_size_mb') * 1024),
            ],
        ]);
        $post = $this->getPost($request->input('post_serial'));
        $path = $this->moveUploadedFile($request->file('file'), $post->serial);
        $isImage = strpos($request->file('file')->getMimeType(), 'image') !== false;

        $pathId = $this->putFileInfoCache($path, $isImage);

        return response()->json([
            'path_id' => $pathId,
            'url' => \Storage::url($path),
            'is_image' => $isImage
        ]);
    }

    public function reportRemoved(Request $request)
    {
        $request->validate([
            'element_id' => 'required|integer',
        ]);

        if(\Cache::has('imgur_image_removed_' . $request->element_id)){
            return response()->json();
        }
        $this->elementService->reportImgureImageRemoved(Element::findOrFail($request->element_id));
        \Cache::put('imgur_image_removed_' . $request->element_id, true, now()->addHour());

        return response()->json();
    }

    protected function putFileInfoCache($path, $isImage)
    {
        $pathId = hash('sha256', $path);
        \Cache::put($pathId, [
            'path' => $path,
            'is_image' => $isImage,
        ], now()->addMinutes(10));
        return $pathId;
    }

    protected function getFileInfoCache($pathId)
    {
        return \Cache::get($pathId);
    }

    public function delete(Request $request, Element $element)
    {
        /** @see ElementPolicy::delete() */
        $this->authorize('delete', $element);
        $this->elementService->delete($element);
        return response()->json();
    }


    protected function tryStroeYoutubeEmbed(Request $request, string $embedCode, array $params = [])
    {
        $guess = new ElementSourceGuess();
        $guess->guessYoutubeEmbed($embedCode);
        if(!$guess->isYoutubeEmbed){
            return null;
        }
        $post = $this->getPost($request->post_serial);
        $element = $this->elementService->storeYoutubeEmbed($embedCode, $post, $params);
        throw_if($element == null, \Exception::class, "Invalid Youtube Embed");
        return $element;

    }
    protected function storeYoutubeEmbed(Request $request, string $embedCode, array $params = [])
    {
        $post = $this->getPost($request->post_serial);
        $element = $this->elementService->storeYoutubeEmbed($embedCode, $post, $params);
        return $element;
    }

    protected function isEmbedString($string)
    {
        $guess = new ElementSourceGuess();
        $guess->guessYoutubeEmbed($string);
        return $guess->isYoutubeEmbed;
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }

    protected function attemptUploadRateLimit(Request $request)
    {
        // 30MB per minute
        $rateLimit = config('setting.upload_media_size_mb_at_a_time') * 1024 * 1024;
        $timeMinuteLimit = 1;
        $rateLimitKey = "upload_rate_limit_size_" . Auth::id();
        $rateLimitValue = \Cache::get($rateLimitKey, 0);
        if($rateLimitValue > $rateLimit){
            throw new \Exception("Rate limit exceeded");
        }
        $rateLimitValue += $request->file('file')->getSize();
        \Cache::put($rateLimitKey, $rateLimitValue, now()->addMinutes($timeMinuteLimit));

        // 50 files per minute
        $fileLimit = config('setting.upload_media_file_count_at_a_time');
        $fileLimitKey = "upload_rate_limit_count" . Auth::id();
        $fileLimitValue = \Cache::get($fileLimitKey, 0);
        $fileLimitValue += 1;
        if($fileLimitValue > $fileLimit){
            throw new \Exception("Rate limit exceeded");
        }
        \Cache::put($fileLimitKey, $fileLimitValue, now()->addMinutes($timeMinuteLimit));
    }

    public function checkMediaSize($url)
    {
        $maxSizeMb = config('setting.upload_media_file_size_mb');
        $maxSizeBytes = $maxSizeMb * 1024 * 1024;

        // Fetch headers
        $headers = get_headers($url, 1);

        if ($headers === false) {
            return false;
        }

        // Check content type
        $contentType = $headers['Content-Type'] ?? '';
        $isImage = strpos($contentType, 'image/') === 0;
        $isVideo = strpos($contentType, 'video/') === 0;

        if ($isImage || $isVideo) {
            // Check file size
            $contentLength = $headers['Content-Length'] ?? 0;
            if ($contentLength > $maxSizeBytes) {
                return false;
            }
        }

        return true;

    }
}
