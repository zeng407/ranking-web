@extends('layouts.app')

@section('content')
    <edit-post
        play-game-route="{{route('game.show', $serial)}}"

        show-post-endpoint="{{route('api.post.show', $serial)}}"
        get-elements-endpoint="{{route('api.post.elements', $serial)}}"
        get-rank-endpoint="{{route('api.post.rank', $serial)}}"
        update-post-endpoint="{{route('api.post.update', $serial)}}"

        update-element-endpoint="{{route('api.element.update', '_id')}}"
        delete-element-endpoint="{{route('api.element.delete', '_id')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        create-image-url-element-endpoint="{{route('api.element.create-image-url')}}"
        create-video-youtube-element-endpoint="{{route('api.element.create-video-youtube')}}"
        create-video-url-element-endpoint="{{route('api.element.create-video-url')}}"
    >
    </edit-post>
@endsection
