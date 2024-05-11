@extends('admin.layouts.app', ['title' => __('管理後台 - 首頁輪播')])

@section('content')
    <div class="container">
        <h1 class="mt-3">{{ __('首頁輪播') }}</h1>
        <edit-home-carousel
            index-endpoint="{{ route('admin.api.home-carousel.index') }}"
            delete-endpoint="{{ route('admin.api.home-carousel.delete', '_id') }}"
            update-endpoint="{{ route('admin.api.home-carousel.update', '_id') }}"
            reorder-endpoint="{{ route('admin.api.home-carousel.reorder') }}"
            create-endpoint="{{ route('admin.api.home-carousel.create') }}"
        >
        </edit-home-carousel>
    </div>
@endsection
