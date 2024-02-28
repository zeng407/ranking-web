<?php

namespace App\Http\Controllers\Admin;

use App\Http\Controllers\Controller;
use Illuminate\Http\Request;
use App\Models\User;
use Illuminate\Pagination\Paginator;

class UserController extends Controller
{
    public function index()
    {
        $users = User::orderByDesc('id')->paginate(10);
        return view('admin.user.index', compact('users'));
    }

    public function search(Request $request)
    {
        $search = $request->input('name');
        if(empty($search)) {
            $users = new Paginator([], 10);
            return view('admin.user.search', compact('users'));
        }
        $users = User::with('roles')
            ->where('name', 'like', '%' . $search . '%')
            ->orWhere('email', 'like', '%' . $search . '%')
            ->orderByDesc('id')
            ->paginate(10);
        return view('admin.user.search', compact('users'));
    }
}
