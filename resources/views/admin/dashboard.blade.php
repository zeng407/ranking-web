@extends('admin.layouts.app', ['title' => '管理後台'])

@section('content')
    <div class="container">
    
        <div class="row justify-content-center">
            <div class="col-md-8">
                <div class="card">
                    <div class="card-header">{{ __('管理後台') }}</div>
    
                    <div class="card-body">
                        {{ __('您已登入!') }}
                    </div>
                </div>
            </div>
        </div>
        
        
    </div>
@endsection
