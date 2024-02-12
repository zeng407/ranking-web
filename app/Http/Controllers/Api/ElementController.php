<?php

namespace App\Http\Controllers\Api;

use App\Enums\ApiResponseCode;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Models\Element;
use App\Models\Post;
use App\Policies\ElementPolicy;
use App\Services\ElementService;
use App\Services\YoutubeService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;

class ElementController extends Controller
{
    protected $youtube;

    protected $elementService;

    public function __construct(YoutubeService $service, ElementService $elementService)
    {
        $this->middleware('auth');
        $this->youtube = $service;
        $this->elementService = $elementService;
    }

    public function createImage(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'file' => 'required|image|max:8192',
        ]);

        $post = $this->getPost($request->input('post_serial'));

        // check elements count
        if ($post->elements()->count() > config('setting.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        $path = Auth::id() . '/' . $request->input('post_serial');
        $element = $this->elementService->storePublicImage($request->file('file'), $path, $post);
        return PostElementResource::make($element);
    }

    public function batchCreate(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'url' => 'required|string',
        ]);
        $urls = $request->input('url');

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
            \Log::debug("{$post->serial} OVER_ELEMENT_SIZE");
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        try {
            $path = Auth::id() . '/' . $request->post_serial;
            $elements = [];
            $errors = [];
            foreach ($urls as $urlStr) {
                $parts = explode(" ", $urlStr,2);
                $url = $parts[0] ?? "";
                $title = $parts[1] ?? "";

                $elementParams = [];
                if($title){
                    $elementParams['title'] = substr($title,0,config('setting.element_title_size'));
                }
                $element = $this->elementService->massStore($url, $path, $post, $elementParams);
                \Log::debug("massStore return");
                \Log::debug($element);
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
            \Log::debug("NO_ELEMENT_CREATED");
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
            'title' => ['sometimes', 'string', 'max:' . config('setting.element_title_size')],
            'video_start_second' => 'sometimes|integer',
            'video_end_second' => 'sometimes|integer',
        ]);

        $element->update($data);

        return PostElementResource::make($element);
    }

    public function delete(Request $request, Element $element)
    {
        /** @see ElementPolicy::delete() */
        $this->authorize('delete', $element);
        $this->elementService->delete($element);
        return response()->json();
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }
}
