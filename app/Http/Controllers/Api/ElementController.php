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
            //url string to array and trim url
            $urls = array_unique(explode(',', $urls));
            $urls = array_filter(array_map(function ($url) {
                return trim($url);
            }, $urls));

            $urlCount = 0;
            foreach ($urls as $url) {
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
                'error_url' => $url,
            ]);
        }

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() + $urlCount > config('setting.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE, 422);
        }

        try {
            $path = Auth::id() . '/' . $request->post_serial;
            $elements = [];
            $errors = [];
            foreach ($urls as $url) {
                if ($this->elementService->getExistsElement($url, $post)) {
                    \Log::debug("skip exists source $url");
                    continue;
                }

                $element = $this->elementService->massStore($url, $path, $post);
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
            return api_response(ApiResponseCode::INVALID_URL, 422, [
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

        $element->posts()->detach();
        $element->delete();

        return response()->json();
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }
}
