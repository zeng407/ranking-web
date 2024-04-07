@extends('layouts.app',['title' => __('My Votes')])

@section('header')
  <meta name="robots" content="noindex"/>
@endsection

@section('content')
    <index-post
        get-posts-endpoint="{{route('api.post.index')}}"
        edit-post-route="{{route('post.edit', '_serial')}}"
        create-post-endpoint="{{route('api.post.create')}}"
        max-post-title-length="{{config('setting.post_title_size')}}"
        max-post-description-length="{{config('setting.post_description_size')}}"
    >
    </index-post>
@endsection

@section('footer')
  @include('partial.footer')
@endsection
