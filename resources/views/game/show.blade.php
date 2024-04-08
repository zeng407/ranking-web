@extends('layouts.app', [
    'title' => $post,
    'ogTitle' => $post->title,
    'ogImage' => $element->thumb_url,
    'ogDescription' => $post->description,
    ])

@section('header')
    <script type="application/ld+json">
    {
        "@context": "https://schema.org",
        "@type": "NewsArticle",
        "headline": "{{get_page_title($post->title)}}",
        "datePublished": "{{$post->created_at->toIso8601String()}}",
        "dateModified": "{{$post->updated_at->toIso8601String()}}"
    }
    </script>
@endsection

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
