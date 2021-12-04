@extends('layouts.app')

@section('content')
	<example-component
		index-posts-endpoint="{{route('public-post.index')}}"
        show-post-endpoint="{{route('post.show', '_serial')}}"
	></example-component>
@endsection
