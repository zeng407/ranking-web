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
                                <label class="col-form-label text-md-right">{{ __('標籤') }}</label>
                                <div class="row">
                                    @for ($i = 0; $i < config('setting.post_max_tags'); $i++)
                                        <div class="col-3 mb-1">
                                            <div class="input-group">
                                                <div class="input-group-prepend">
                                                    <span class="input-group-text" id="hashtag">#</span>
                                                </div>
                                                <input class="form-control" name="tags[{{ $i }}]" type="text"
                                                    maxlength="15" aria-label="hashtag" aria-describedby="hashtag"
                                                    value="{{ data_get($post->tags, "$i.name") }}"
                                                    placeholder="第{{ $i + 1 }}個標籤"></input>
                                            </div>
                                        </div>
                                    @endfor
                                </div>
                                <div class="form-group">
                                    <label for="post_policy"
                                        class="col-form-label text-md-right">{{ __('發佈') }}</label>
                                    <select id="post_policy" class="form-control" name="policy[access_policy]">
                                        <option value="{{ \App\Enums\PostAccessPolicy::PUBLIC }}"
                                            @if ($post->post_policy->access_policy == \App\Enums\PostAccessPolicy::PUBLIC) selected @endif>
                                            {{ \App\Enums\PostAccessPolicy::trans(\App\Enums\PostAccessPolicy::PUBLIC) }}
                                        </option>
                                        <option value="{{ \App\Enums\PostAccessPolicy::PRIVATE }}"
                                            @if ($post->post_policy->access_policy == \App\Enums\PostAccessPolicy::PRIVATE) selected @endif>
                                            {{ \App\Enums\PostAccessPolicy::trans(\App\Enums\PostAccessPolicy::PRIVATE) }}
                                        </option>
                                        <option value="{{ \App\Enums\PostAccessPolicy::PASSWORD }}"
                                            @if ($post->post_policy->access_policy == \App\Enums\PostAccessPolicy::PASSWORD) selected @endif>
                                            {{ \App\Enums\PostAccessPolicy::trans(\App\Enums\PostAccessPolicy::PASSWORD) }}
                                        </option>
                                    </select>
                                </div>

                                <div class="form-group">
                                    <label for="user" class="col-form-label text-md-right">{{ __('會員') }}</label>
                                    <input id="user" type="text" class="form-control" name="user"
                                        value="{{ $post->user->name }} ({{ $post->user->email }})" disabled>
                                </div>

                                <div class="form-group row">
                                  <div class="col-12">
                                    <label>{{ __('瀏覽受限') }}</label>
                                    <div>
                                      <label class="btn btn-outline-dark" for="is_censored">是
                                        <input type="radio" id="is_censored" name="is_censored" value="1" @if ($post->is_censored) checked @endif>
                                      </label>
                                      <label class="btn btn-outline-dark" for="is_not_censored">否
                                        <input type="radio" id="is_not_censored" name="is_censored" value="0" @if (!$post->is_censored) checked @endif>
                                      </label>
                                    </div>
                                  </div>
                              </div>

                                <div class="form-group">
                                    <button class="form-control btn btn-outline-primary" type="submit">更新</button>
                                </div>
                            </div>
                        </form>
                        <form action="{{ route('admin.post.delete', $post->id) }}" method="POST">
                            @csrf
                            @method('DELETE')
                            <button class="form-control btn btn-outline-danger" type="submit" onclick="return confirm('Are you sure you want to delete this item?')">
                                <i class="fas fa-trash"></i> {{ __('Delete') }}
                            </button>
                        </form>
                    </div>
                </div>
            </div>
        </div>

        <h1 class="mt-4">{{ __('素材管理') }}</h1>
        <element-form index-element-route="{{ route('admin.api.element.index', $post->id) }}"
            update-element-route="{{ route('admin.api.element.update', [$post->id, 'element_id']) }}"
            delete-element-route="{{ route('admin.api.element.delete', [$post->id, 'element_id']) }}"></element>
    </div>
@endsection
