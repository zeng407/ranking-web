<?php

namespace App\Http\Controllers\Api;

use App\Enums\ApiResponseCode;
use App\Enums\ElementType;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Models\Element;
use App\Models\Post;
use App\Policies\ElementPolicy;
use App\Services\ElementService;
use App\Services\ElementSourceGuess;
use App\Services\YoutubeService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class ElementController extends Controller
{
    protected $youtube;

    protected $elementService;

    public function __construct(YoutubeService $service, ElementService $elementService)
    {
        $this->youtube = $service;
        $this->elementService = $elementService;
    }

    public function createImage(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required|string',
            'file' => 'required|image|max:8192',
        ]);

        $post = $this->getPost($request->input('post_serial'));

        // check elements count
        if ($post->elements()->count() >= config('setting.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        $path = $request->input('post_serial');
        $element = $this->elementService->storePublicImage($request->file('file'), $path, $post);
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
        if ($this->isEmbedString($urls)) {
            $element = $this->storeYoutubeEmbed($request, $urls);
            if($element === null){
                return api_response(ApiResponseCode::INVALID_URL, 422);
            }
            return PostElementResource::collection([$element]);
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

            // if the url is a youtube video embed and iframe
            if ($this->isEmbedString($data['url'])) {
                $element = $this->storeYoutubeEmbed($request, $data['url'], [
                    'old_source_url' => $element->source_url,
                ]);
                if($element === null){
                    return api_response(ApiResponseCode::INVALID_URL, 422);
                }
                return PostElementResource::make($element);
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
            $path = \Cache::get($data['path_id']);
            if($path == null){
                return api_response(ApiResponseCode::INVALID_PATH, 422);
            }

            if(\Storage::exists($path)){
                $url = \Storage::url($path);
                $data['thumb_url'] = $url;
                $data['source_url'] = $url;
                $data['type'] = ElementType::IMAGE;
                $data['video_source'] = null;
                $data['video_id'] = null;
                $data['video_duration_second'] = null;
                $data['video_start_second'] = null;
                $data['video_end_second'] = null;
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
            'file' => 'required|image|max:8192',
        ]);
        $post = $this->getPost($request->input('post_serial'));
        $path = $this->elementService->moveUploadedFile($request->file('file'), $post->serial);
        $pathId = hash('sha256', $path);
        \Cache::put($pathId, $path, now()->addMinutes(10));
        return response()->json([
            'path_id' => $pathId,
            'url' => \Storage::url($path),
        ]);
    }

    public function delete(Request $request, Element $element)
    {
        /** @see ElementPolicy::delete() */
        $this->authorize('delete', $element);
        $this->elementService->delete($element);
        return response()->json();
    }

    protected function storeYoutubeEmbed(Request $request, string $urls, array $params = [])
    {
        $post = $this->getPost($request->post_serial);
        $element = $this->elementService->storeYoutubeEmbed($urls, $post, $params);
        return $element;
    }

    protected function isEmbedString($string)
    {
        return preg_match('/https:\/\/www\.youtube\.com\/embed\/([a-zA-Z0-9_-]+)/', $string) && 
            preg_match('/^<iframe.*?src="(.*?)".*?<\/iframe>$/', $string);
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }
}
