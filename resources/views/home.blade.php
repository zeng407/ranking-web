@extends('layouts.app')

@section('content')
    <post
        index-posts-endpoint="{{route('api.public-post.index')}}"
        play-game-route="{{route('game.show', '_serial')}}"
    ></post>
@endsection
