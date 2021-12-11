@extends('layouts.app')

@section('content')
    <create-post
        get-posts-endpoint="{{route('api.post.index')}}"
        show-post-endpoint="{{route('api.post.show', '_serial')}}"
        create-post-endpoint="{{route('api.post.create')}}"
        update-post-endpoint="{{route('api.post.update', '_serial')}}"
        update-element-endpoint="{{route('api.element.update', '_id')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        create-video-element-endpoint="{{route('api.element.create-video')}}"
    >

    </create-post>
@endsection
