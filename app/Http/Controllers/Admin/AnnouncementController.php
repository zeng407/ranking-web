<?php

namespace App\Http\Controllers\Admin;

use App\Helper\CacheService;
use App\Http\Controllers\Controller;
use Illuminate\Http\Request;

class AnnouncementController extends Controller
{
    public function index()
    {
        $announcement = CacheService::rememberAnnouncement();
        return view('admin.announcement.index', ['announcement' => $announcement]);
    }

    public function create(Request $request)
    {
        $request->validate([
            'content' => 'required|string',
            'minutes' => 'nullable|integer',
            'image_url' => 'nullable|url',
        ]);
        $minutes = request()->input('minutes', 60);
        $data = [
            'id' => \Str::uuid(),
            'content' => request()->input('content'),
            'image_url' => request()->input('image_url'),
            'created_at' => now()->toDateTimeString(),
            'keep_minutes' => $minutes,
        ];
        CacheService::rememberAnnouncement($data, $minutes, true);
        return redirect()->route('admin.announcement.index');
    }
}
