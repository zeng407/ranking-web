@extends('layouts.app', [
    'title' => $post,
    'ogTitle' => $post->title,
    'ogImage' => optional($ranks->pop()->element)->thumb_url,
    'ogDescription' => $post->description])

@section('content')
    <game
        post-serial="{{$serial}}"
        get-rank-route="{{route('game.rank', '_serial')}}"
        get-game-setting-endpoint="{{route('api.game.setting', $serial)}}"
        next-round-endpoint="{{route('api.game.next-round', '_serial')}}"
        create-game-endpoint="{{route('api.game.create')}}"
        vote-game-endpoint="{{route('api.game.vote')}}"
    ></game>
@endsection
