@extends('admin.layouts.app', ['title' => __('管理後台 - 會員查詢')])

@section('content')
    <div class="container">
        <div class="card">
            <div class="card-header">
                <h5>{{ __('會員查詢') }}</h1>
            </div>
            <div class="card-body">
                <form action="{{ route('admin.user.search') }}" method="GET">
                    <div class="col-3">
                        <div class="form-group">
                            <label for="name">名稱或Email</label>
                            <input type="text" name="name" id="name" class="form-control" placeholder=""
                                value="{{ request('name') }}">
                        </div>
                    </div>
                    <div class="col-12">
                        <button type="submit" class="btn btn-primary fa-pull-right">搜尋</button>
                        <a href="{{ request()->url() }}" class="btn btn-outline-secondary fa-pull-right">重設</a>
                    </div>
                </form>
            </div>
        </div>
        @if ($users->count() > 0)
            <div class="mt-3">
                <user-form 
                    :users="{{ json_encode($users) }}" 
                    ban-user-route="{{ route('admin.user.ban', 'user_id') }}"
                    unban-user-route="{{ route('admin.user.unban', 'user_id') }}">
                </user-form>
                <div class="text-center">
                    {{ $users->appends(request()->except('page'))->links() }}
                </div>
            </div>
        @endif
    </div>
@endsection
