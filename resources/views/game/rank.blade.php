@extends('layouts.app')

@section('content')
    <rank
        post-serial="{{$serial}}"
        get-rank-endpoint="{{route('api.rank.index', $serial)}}"
        get-rank-report-endpoint="{{route('api.rank.report', $serial)}}"
    >

    </rank>
@endsection
