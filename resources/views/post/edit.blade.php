@extends('layouts.app')

@section('content')
    <edit-post
        show-post-endpoint="{{route('api.post.show', $serial)}}"
        get-elements-endpoint="{{route('api.post.elements', $serial)}}"
        update-post-endpoint="{{route('api.post.update', $serial)}}"

        update-element-endpoint="{{route('api.element.update', '_id')}}"
        delete-element-endpoint="{{route('api.element.delete', '_id')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        create-video-element-endpoint="{{route('api.element.create-video')}}"
    >
    </edit-post>
@endsection
