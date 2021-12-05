@extends('layouts.app')

@section('content')
    <rank
        post-serial="{{$serial}}"
        show-rank-endpoint="{{route('api.rank.show', $serial)}}"
    ></rank>
@endsection
