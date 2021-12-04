<?php

namespace App\Http\Controllers\Api;

use App\Enums\ElementType;
use App\Http\Controllers\Controller;
use App\Models\Post;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;

class ElementController extends Controller
{
    public function __construct()
    {
        $this->middleware('auth');
    }



    public function createImage(Request $request)
    {
        $request->validate([
            'post_serial' => 'required',
            'file' => 'required|image',
        ]);

        $post = $this->getPost($request->post_serial);

        $saveDir = Auth::id() . '/' . $request->post_serial;
        $file = $request->file('file');
        $path = $file->store($saveDir);

        //todo sign path
        $url = $path;

        //todo make thumb
        $thumb = $url;

        $element = $post->elements()->create([
            'path' => $path,
            'source_url' => $url,
            'thumb_url' => $thumb,
            'type' => ElementType::IMAGE,
            'title' => pathinfo($file->getClientOriginalName(),PATHINFO_FILENAME)
        ]);

        return $element;
    }

    public function createVideo(Request $request)
    {
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

        return $element;
    }

    protected function getPost($serial): Post
    {
        return Post::where('user_id', Auth::id())
            ->where('serial', $serial)
            ->firstOrFail();
    }
}
