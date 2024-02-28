@extends('admin.layouts.app', ['title' => __('管理後台 - 貼文管理')])

@section('content')
    <div class="container">
        <h1 class="mt-3">{{ __('貼文管理') }} </h1>
        <div class="row">
            <div class="col-12">
                <div class="card">
                    <div class="card-header">
                        {{ __('貼文詳細資訊') }}
                    </div>
                    <div class="card-body">
                        <form action="{{ route('admin.post.update', $post->id) }}" method="POST">
                            @csrf
                            @method('PUT')
                            <div class="form-group">
                                <label for="id" class="col-form-label text-md-right">{{ __('ID') }}</label>
                                <input id="id" type="text" class="form-control" name="id"
                                    value="{{ $post->id }}" disabled>
                            </div>
                            <div class="form-group">
                                <label for="title" class="col-form-label text-md-right">{{ __('標題') }}</label>
                                <input id="title" type="text" class="form-control" name="title"
                                    value="{{ $post->title }}">
                            </div>
                            <div class="form-group">
                                <label for="description" class="col-form-label text-md-right">{{ __('描述') }}</label>
                                <textarea id="description" class="form-control" name="description">{{ $post->description }}</textarea>
                            </div>
                            <div class="form-group">
                                <label for="post_policy" class="col-form-label text-md-right">{{ __('發佈') }}</label>
                                <select id="post_policy" class="form-control" name="policy[access_policy]">
                                    <option value="{{ \App\Enums\PostAccessPolicy::PUBLIC }}"
                                        @if ($post->post_policy->access_policy == \App\Enums\PostAccessPolicy::PUBLIC) selected @endif>
                                        {{ \App\Enums\PostAccessPolicy::trans(\App\Enums\PostAccessPolicy::PUBLIC) }}</option>
                                    <option value="{{ \App\Enums\PostAccessPolicy::PRIVATE }}"
                                        @if ($post->post_policy->access_policy == \App\Enums\PostAccessPolicy::PRIVATE) selected @endif>
                                        {{ \App\Enums\PostAccessPolicy::trans(\App\Enums\PostAccessPolicy::PRIVATE) }}</option>
                                </select>
                            </div>

                            <div class="form-group">
                                <label for="user" class="col-form-label text-md-right">{{ __('會員') }}</label>
                                <input id="user" type="text" class="form-control" name="user"
                                    value="{{ $post->user->name }} ({{ $post->user->email }})" disabled>
                            </div>

                            <div class="form-group">
                                <button class="form-control btn btn-outline-danger" type="submit">更新</button>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        </div>

        <h1 class="mt-4">{{ __('素材管理') }}</h1>
        <element-form 
            index-element-route="{{ route('admin.api.element.index', $post->id) }}"
            update-element-route="{{ route('admin.api.element.update', [$post->id, 'element_id']) }}"
            delete-element-route="{{ route('admin.api.element.delete', [$post->id, 'element_id']) }}"></element>
    </div>
@endsection
