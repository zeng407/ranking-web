@extends('layouts.app')

@section('content')
	<post
		index-posts-endpoint="{{route('api.public-post.index')}}"
        show-post-endpoint="{{route('post.show', '_serial')}}"
	></post>
@endsection
