<?php

namespace App\Http\Controllers\Profile;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use Storage;

class ProfileController extends Controller
{
    public function index()
    {
        return view('account.profile.index');
    }

    public function update(Request $request)
    {
        $validatedData = $request->validate([
            'name' => ['sometimes', 'required', 'string', 'max:' . config('setting.user_name_max_size'), 'min:' . config('setting.user_name_min_size')],
            'avatar_url' => ['sometimes', 'required', 'image', 'max:' . config('setting.avatar_max_size')],
        ]);

        if(isset($validatedData['name']) && $validatedData['name'] !== $request->user()->name) {
            if($request->user()->name_updated_at === null || today()->diffInDays($request->user()->name_updated_at->toDateString()) >= config('setting.name_change_duration')){
                $validatedData['name_updated_at'] = now();
            } else {
                return redirect()->back()->with('error', __('You can change your name only once in :days days', ['days' => config('setting.name_change_duration')]));
            }
        }
        
        if ($request->hasFile('avatar')) {
            $path = $request->file('avatar')->store('avatars');
            if (!$path) {
                return redirect()->back()->with('error', __('Failed to upload avatar'));
            }
            $validatedData['avatar_url'] = Storage::url($path);
        }
        
        $request->user()->update($validatedData);

        return redirect()->back()->with('success', __('Profile updated successfully'));
    }

    public function updatePassword(Request $request)
    {
        $validatedData = $request->validate([
            'current_password' => ['required', 'password'],
            'new_password' => ['required', 'confirmed', 'min:' . config('setting.password_min_size')],
        ]);

        $request->user()->update(['password' => bcrypt($validatedData['new_password'])]);

        return redirect()->back()->with('success', __('Password updated successfully'));
    }

    public function initPassword(Request $request)
    {
        $validatedData = $request->validate([
            'new_password' => ['required', 'confirmed', 'min:' . config('setting.password_min_size')],
        ]);

        if($request->user()->password !== '') {
            return redirect()->back()->with('error', __('Please reset your password.'));
        }

        $request->user()->update(['password' => bcrypt($validatedData['new_password'])]);

        return redirect()->back()->with('success', __('Password updated successfully'));
    }
}