@extends('admin.layouts.app', ['title' => __('管理後台 - 會員列表')])

@section('content')
    <div class="container">
        <h1 class="mt-3">{{ __('會員列表') }}</h1>
        <table class="table table-bordered w-100">
            <thead>
                <tr>
                    <th>UID</th>
                    <th>Email</th>
                    <th>名稱</th>
                    <th>有效貼文</th>
                    <th>建立日</th>
                </tr>
            </thead>
            <tbody>
                @foreach ($users as $user)
                    <tr>
                        <td>{{ $user->id }}</td>
                        <td>{{ $user->email }}</td>
                        <td>{{ $user->name }}</td>
                        <td>{{ $user->posts()->count() }}</td>
                        <td>{{ $user->created_at->toDateString() }} <span
                                class="badge badge-secondary">{{ $user->created_at->diffForHumans() }}</span></td>
                    </tr>
                @endforeach
            </tbody>
        </table>
        <!-- Pagination links -->
        <div class="text-center">
            {{ $users->links() }}
        </div>
    </div>
@endsection
