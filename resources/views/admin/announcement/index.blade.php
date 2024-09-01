@extends('admin.layouts.app', ['title' => __('管理後台 - 公告')])

@section('content')
    <div class="container">
        <h1 class="mt-3">{{ __('公告') }}</h1>
        <div class="card">
            <div class="card-header">
                <h5>{{ __('新增公告') }}</h5>
            </div>
            <div class="card-body">
                <form action="{{ route('admin.announcement.create') }}" method="POST">
                    @csrf
                    <div class="form-group">
                        <label for="content">{{ __('內容') }}</label>
                        <textarea name="content" id="content" class="form-control" rows="5" required></textarea>
                    </div>
                    <div class="form-group">
                      <label for="content">{{ __('圖片URL') }}</label>
                      <input type="text" name="image_url" id="image_url" class="form-control">
                  </div>

                    <div class="form-group">
                        <label for="minutes">{{ __('公告時間(分鐘)') }}</label>
                        <input type="int" name="minutes" id="minutes" class="form-control" required>
                    </div>
                    <button type="submit" class="btn btn-primary">{{ __('新增') }}</button>
                </form>
            </div>
            @if ($announcement && !empty($announcement['content']))
                <div class="card-body">
                    <h5 class="text-center">
                      {!! nl2br($announcement['content']) !!}
                    </h5>
                    @if($announcement['image_url'])
                    <div class="text-center">
                      <img src="{{$announcement['image_url']}}" width="auto" height="400px">
                    </div>
                    @endif
                    <div class="text-right">
                      @if(isset($announcement['created_at']))
                      公告時間：{{$announcement['created_at']}}<br>
                      @endif
                      @if(isset($announcement['keep_minutes']))
                      持續：{{$announcement['keep_minutes']}}分鐘
                      @endif
                    </div>
                </div>
            @endif
        </div>
    </div>
@endsection
