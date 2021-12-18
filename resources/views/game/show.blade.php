@extends('layouts.app')

@section('content')
    <game
        post-serial="{{$serial}}"
        rank-route="{{route('game.rank', $serial)}}"
        get-game-setting-endpoint="{{route('api.game.setting', $serial)}}"
        next-round-endpoint="{{route('api.game.next-round', '_serial')}}"
        create-game-endpoint="{{route('api.game.create')}}"
        vote-game-endpoint="{{route('api.game.vote')}}"
    ></game>
@endsection
