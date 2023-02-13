@extends('layouts.app', ['post' => $post])

@section('header')
  <meta name="robots" content="noindex"/>
@endsection

@section('content')
    <edit-post
        :config="{{json_encode(config('setting'))}}"
        play-game-route="{{route('game.show', $serial)}}"

        show-post-endpoint="{{route('api.post.show', $serial)}}"
        get-elements-endpoint="{{route('api.post.elements', $serial)}}"
        get-rank-endpoint="{{route('api.post.rank', $serial)}}"
        update-post-endpoint="{{route('api.post.update', $serial)}}"

        update-element-endpoint="{{route('api.element.update', '_id')}}"
        delete-element-endpoint="{{route('api.element.delete', '_id')}}"
        create-image-element-endpoint="{{route('api.element.create-image')}}"
        batch-create-endpoint="{{route('api.element.batch-create')}}"
    >
    </edit-post>

@endsection

@section('footer')
  @include('partial.footer')
@endsection
