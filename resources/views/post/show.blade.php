@extends('layouts.app')

@section('content')
    <game
        post-serial="{{$serial}}"
        rank-route="{{route('post.rank', $serial)}}"
        show-game-endpoint="{{route('api.game.show', '_serial')}}"
        create-game-endpoint="{{route('api.game.create')}}"
        vote-game-endpoint="{{route('api.game.vote')}}"
    ></game>
@endsection
