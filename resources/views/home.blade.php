@extends('layouts.app')

@section('content')
    <home
        index-posts-endpoint="{{route('api.public-post.index')}}"
        play-game-route="{{route('game.show', '_serial')}}"
    ></home>
@endsection
