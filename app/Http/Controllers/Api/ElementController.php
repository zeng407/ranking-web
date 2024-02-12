<?php

namespace App\Http\Controllers\Api;

use App\Enums\ApiResponseCode;
use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Http\Controllers\Controller;
use App\Http\Resources\MyPost\PostElementResource;
use App\Models\Element;
use App\Models\Post;
use App\Policies\ElementPolicy;
use App\Services\ElementService;
use App\Services\YoutubeService;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;
use Illuminate\Validation\Rule;

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

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() > config('post.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE,422);
        }

        $path = Auth::id() . '/' . $request->post_serial;
        $element = $this->elementService->storePublic($request->file('file'), $path, $post);
        return PostElementResource::make($element);
    }

    public function createImageUrl(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'url' => 'required|url|string',
        ]);

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() > config('post.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE,422);
        }

        try {
            $path = Auth::id() . '/' . $request->post_serial;
            $element = $this->elementService->storePublicFromImageUrl($request->input('url'), $path, $post);
            if(!$element){
                return api_response(ApiResponseCode::INVALID_URL,422);
            }
        }catch (\Exception $exception){
            report($exception);
            return api_response(ApiResponseCode::INVALID_URL,422);
        }
        return PostElementResource::make($element);
    }

    public function createVideoYoutube(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'url' => 'required|string',
            'video_start_second' => 'nullable|integer',
            'video_end_second' => 'nullable|integer'
        ]);

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() > config('post.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE,422);
        }

        try {
            $video = $this->youtube->query($request->url);
            if (!$video) {
                return api_response(ApiResponseCode::INVALID_URL,422);
            }

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

            $hourPart = (int)$parts[1];
            $minutePart = (int)$parts[2];
            $secondPart = (int)$parts[3];
            $second = $hourPart * 3600 + $minutePart * 60 + $secondPart;
        } catch (\Exception $exception) {
            report($exception);

            return api_response(ApiResponseCode::INVALID_URL,422);
        }

        $element = $post->elements()->create([
            'source_url' => $request->url,
            'thumb_url' => $thumbUrl,
            'title' => $title,
            'type' => ElementType::VIDEO,
            'video_source' => VideoSource::YOUTUBE,
            'video_id' => $id,
            'video_duration_second' => $second,
            'video_start_second' => $request->video_start_second,
            'video_end_second' => $request->video_end_second
        ]);

        return PostElementResource::make($element);
    }

    public function createVideoUrl(Request $request)
    {
        /** @see ElementPolicy::create() */
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'url' => 'required|url|string',
        ]);

        $post = $this->getPost($request->post_serial);

        // check elements count
        if ($post->elements()->count() > config('post.post_max_element_count')) {
            return api_response(ApiResponseCode::OVER_ELEMENT_SIZE,422);
        }

        try {
            $path = Auth::id() . '/' . $request->post_serial;
            $element = $this->elementService->tryStorePublicVideoUrl($request->input('url'), $path, $post);
            if(!$element){
                return api_response(ApiResponseCode::INVALID_URL,422);
            }
        }catch (\Exception $exception){
            report($exception);
            return api_response(ApiResponseCode::INVALID_URL,422);
        }
        return PostElementResource::make($element);
    }

    public function update(Request $request, Element $element)
    {
        /** @see ElementPolicy::update() */
        $this->authorize('update', $element);

        $data = $request->validate([
            'title' => ['sometimes', 'string', 'max:' . config('post.title_size')],
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
