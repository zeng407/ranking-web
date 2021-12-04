@extends('layouts.app')

@section('content')
    <game
        post-serial="{{$serial}}"
        rank-route="{{route('post.rank', $serial)}}"
        show-game-endpoint="{{route('game.show', '_serial')}}"
        create-game-endpoint="{{route('game.create')}}"
        vote-game-endpoint="{{route('game.vote')}}"
    ></game>
@endsection
