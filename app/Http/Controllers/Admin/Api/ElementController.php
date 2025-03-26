<?php

namespace App\Http\Controllers\Admin\Api;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use App\Models\Element;
use App\Models\Post;
use App\Services\ElementService;

class ElementController extends Controller
{
    public function indexElement($postId)
    {
        $post = Post::findOrFail($postId);
        $elements = $post->elements;
        return response()->json($elements);
    }

    public function updateElement(Request $request, $postId, $elementId)
    {
        $data = $request->validate([
            'title' => ['sometimes', 'string', 'max:' . config('setting.element_title_size')],
        ]);
        $element = Element::findOrFail($elementId);
        $element->update($data);
        return response()->json();
    }

    public function deleteElement($postId, $elementId)
    {
        $element = Element::findOrFail($elementId);
        app(ElementService::class)->delete($element);
        return response()->json();
    }
}
