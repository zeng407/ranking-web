<?php

namespace App\Http\Controllers\Admin\Api;

use App\Http\Controllers\Controller;
use App\Models\HomeCarouselItem;
use App\Services\HomeCarouselService;
use Illuminate\Http\Request;

class HomeCarouselController extends Controller
{
    protected HomeCarouselService $service;

    public function __construct(HomeCarouselService $service)
    {
        $this->service = $service;
    }

    public function index(Request $request)
    {
        return HomeCarouselItem::orderBy('is_active')->orderBy('position')->get();
    }

    public function create(Request $request)
    {
        $data = $request->validate([
            'type' => 'required|string|in:video',
            'title' => 'nullable|string',
            'description' => 'nullable|string',
            'image_url' => 'nullable|url|required_without:video_url',
            'video_url' => 'nullable|url|required_without:image_url',
            'video_start_second' => 'nullable|integer',
            'video_end_second' => 'nullable|integer',
            'is_active' => 'nullable|boolean'
        ]);

        $item = $this->service->createHomeCarouselItem($data);

        if(!$item) {
            return response()->json(['message' => 'Invalid data'], 400);
        }

        return response()->json($item, 201);
    }
    
    public function reorder(Request $request)
    {
        $request->validate([
            'items' => 'required|array',
            'items.*.id' => 'required|integer',
            'items.*.position' => 'required|integer',
        ]);

        $items = $request->input('items');
        foreach($items as $item) {
            HomeCarouselItem::where('id', $item['id'])->update(['position' => $item['position']]);
        }

        return response()->json(['message' => 'Items reordered']);
    }

    public function delete($itemId)
    {
        $item = HomeCarouselItem::find($itemId);
        if(!$item) {
            return response()->json(['message' => 'Item not found'], 404);
        }

        $item->delete();
        return response()->json(['message' => 'Item deleted']);
    }

    public function update(Request $request, $itemId)
    {
        $data = $request->validate([
            'title' => 'nullable|string',
            'description' => 'nullable|string',
            'video_start_second' => 'nullable|integer',
            'video_end_second' => 'nullable|integer',
            'is_active' => 'nullable|boolean'
        ]);

        $item = HomeCarouselItem::find($itemId);
        if(!$item) {
            return response()->json(['message' => 'Item not found'], 404);
        }

        $item->update($data);
        return response()->json($item);
    }
}
