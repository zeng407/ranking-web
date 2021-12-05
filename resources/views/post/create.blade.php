@extends('layouts.app')

@section('content')
    <create-post
        get-post-endpoint="{{route('api.post.index')}}"
        create-post-endpoint="{{route('api.post.create')}}"
        update-post-endpoint="{{route('api.post.update', '_serial')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        create-video-element-endpoint="{{route('api.element.create-video')}}"
    >

    </create-post>
@endsection
