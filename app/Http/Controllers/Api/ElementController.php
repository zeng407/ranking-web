<?php

namespace App\Http\Controllers\Api;

use App\Enums\ElementType;
use App\Http\Controllers\Controller;
use App\Http\Resources\PostElementResource;
use App\Models\Element;
use App\Models\Post;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;
use Illuminate\Validation\Rule;

class ElementController extends Controller
{
    const TITLE_SIZE = 40;

    public function __construct()
    {
        $this->middleware('auth');
    }

    public function createImage(Request $request)
    {
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'file' => 'required|image',
        ]);

        $post = $this->getPost($request->post_serial);

        $saveDir = Auth::id() . '/' . $request->post_serial;
        $file = $request->file('file');
        $path = $file->store($saveDir, 'public');

        //todo sign path
        $url = Storage::url($path);

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => substr(pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME), 0, self::TITLE_SIZE)
        ]);

        return PostElementResource::make($element);
    }

    public function createVideo(Request $request)
    {
        $this->authorize('create', Element::class);

        $request->validate([
            'post_serial' => 'required',
            'url' => 'required|url',
            'video_start_second' => 'nullable|integer',
            'video_end_second' => 'nullable|integer'
        ]);

        $post = $this->getPost($request->post_serial);

        //todo get thumb from youtube
        $thumb = '';

        //todo get title from youtube
        $title = '';

        $element = $post->elements()->create([
            'source_url' => $request->url,
            'thumb_url' => $thumb,
            'title' => $title,
            'type' => ElementType::VIDEO,
            'video_start_second' => $request->video_start_second,
            'video_end_second' => $request->video_end_second
        ]);

        return PostElementResource::make($element);
    }

    public function update(Request $request, $id)
    {
        $element = Element::findOrFail($id);
        $this->authorize('update', $element);

        $data = $request->validate([
            'title' => ['sometimes', 'string', 'max:' . self::TITLE_SIZE],
            'video_start_second' => 'sometimes|integer',
            'video_end_second' => 'sometimes|integer',
        ]);

        $element->update($data);

        return PostElementResource::make($element);
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }
}
