@extends('layouts.app')

@section('content')
    <rank
        post-serial="{{$serial}}"
        show-rank-endpoint="{{route('rank.show', $serial)}}"
    ></rank>
@endsection
