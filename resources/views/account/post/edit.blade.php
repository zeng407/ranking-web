@extends('layouts.app', [
    'title' => $post
  ])

@section('header')
  <meta name="robots" content="noindex"/>
@endsection

@section('content')
    <edit-post
        :config="{{json_encode($config)}}"
        play-game-route="{{route('game.show', $serial)}}"
        game-rank-route={{route('game.rank-embed', $serial)}}

        show-post-endpoint="{{route('api.post.show', $serial)}}"
        get-elements-endpoint="{{route('api.post.elements', $serial)}}"
        get-rank-endpoint="{{route('api.post.rank', $serial)}}"
        update-post-endpoint="{{route('api.post.update', $serial)}}"

        update-element-endpoint="{{route('api.element.update', '_id')}}"
        upload-element-endpoint="{{route('api.element.upload', '_id')}}"
        delete-element-endpoint="{{route('api.element.delete', '_id')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        batch-create-endpoint="{{route('api.element.batch-create')}}"

        get-tags-options-endpoint="{{route('api.tag.index')}}"
        get-comments-endpoint="{{route('api.public-post.comment.index', $serial)}}"
    >
    </edit-post>

@endsection

@section('footer')
  @include('partial.footer')
@endsection
