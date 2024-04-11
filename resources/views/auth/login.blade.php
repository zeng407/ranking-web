@extends('layouts.app',[
    'title' => __('Login')
])
@section('header')
<script src="https://apis.google.com/js/platform.js" async defer></script>
@endsection

@section('content')
<div class="container">
    
    @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])
    <div class="row justify-content-center">
        <div class="col-md-6 col-lg-5">
            <div class="card mt-4">
                <div>
                    <ul class="nav nav-pills mb-3 border-bottom" id="pills-tab" role="tablist">
                        <li class="nav-item w-50" role="presentation">
                            <button class="w-100 nav-link active" id="pills-login-tab" data-toggle="pill" data-target="#pills-login" type="button" role="tab" aria-controls="pills-login" aria-selected="true">
                                    {{ __('Login') }}&nbsp;<i class="fas fa-sign-in-alt"></i>
                            </button>    
                        </li>
                        <li class="nav-item w-50" role="presentation">
                            <button class="w-100 nav-link " id="pills-register-tab" data-toggle="pill" data-target="#pills-register" type="button" role="tab" aria-controls="pills-register" aria-selected="true">
                                    {{ __('Register With Email') }}&nbsp;<i class="fas fa-envelope"></i>
                            </button>    
                        </li>
                    </ul>
                </div>

                <div class="card-body">
                    <div class="tab-content" id="myTabContent">
                        
                        {{-- login --}}
                        <div class="tab-pane fade show active" id="pills-login" role="tabpanel" aria-labelledby="pills-login-tab">
                            {{-- login with email --}}
                            <form method="POST" action="{{ route('login') }}">
                                @csrf
                                <div class="form-group row">
                                    <label for="email" class="col-12 col-form-label">{{ __('E-Mail Address') }}</label>
        
                                    <div class="col-12">
                                        <input id="email" type="email" class="form-control @error('email') is-invalid @enderror" name="email" value="{{ old('email') }}" required autocomplete="email" autofocus>
        
                                        @error('email')
                                            <span class="invalid-feedback" role="alert">
                                                <strong>{{ $message }}</strong>
                                            </span>
                                        @enderror
                                    </div>
                                </div>
        
                                <div class="form-group row">
                                    <label for="password" class="col-12 col-form-label">{{ __('Password') }}</label>
        
                                    <div class="col-12">
                                        <input id="password" type="password" class="form-control @error('password') is-invalid @enderror" name="password" required autocomplete="current-password">
        
                                        @error('password')
                                            <span class="invalid-feedback" role="alert">
                                                <strong>{{ $message }}</strong>
                                            </span>
                                        @enderror
                                    </div>
                                </div>
        
                                <div class="form-group row">
                                    <div class="col-12">
                                        <div class="form-check">
                                            <input class="form-check-input" type="checkbox" name="remember" id="remember" {{ old('remember') ? 'checked' : '' }}>
        
                                            <label class="form-check-label" for="remember">
                                                {{ __('Remember Me') }}
                                            </label>
                                        </div>
                                    </div>
                                </div>
        
                                <div class="form-group row mb-0">
                                    <div class="col-12">
                                        <button type="submit" class="btn btn-outline-light btn-block text-dark border-secondary">
                                            {{ __('Login') }}
                                        </button>

                                    </div>
                                </div>
                            </form>
                            {{-- socialite --}}
                            @include('auth.partial.socialities')
                        </div>
                        {{-- register --}}
                        <div class="tab-pane fade" id="pills-register" role="tabpanel" aria-labelledby="pills-register-tab">
                            <form method="POST" action="{{ route('register') }}">
                                @csrf
    
                                <div class="form-group row">
                                    <label for="name"
                                        class="col-12 col-form-label">{{ __('Nickname') }}</label>
                                    <div class="col-12">
                                        <input id="name" type="text"
                                            class="form-control @error('name') is-invalid @enderror" name="name"
                                            value="{{ old('name') }}" required autocomplete="name" autofocus maxlength="{{config('setting.user_name_max_size')}}">
                                            <small id="nameHelp" class="form-text text-muted">
                                                {{ __('Can be modified later') }}
                                            </small>
                                        @error('name')
                                            <span class="invalid-feedback" role="alert">
                                                <strong>{{ $message }}</strong>
                                            </span>
                                        @enderror
                                    </div>
                                </div>
    
                                <div class="form-group row">
                                    <label for="email"
                                        class="col-12 col-form-label">{{ __('E-Mail Address') }}</label>
    
                                    <div class="col-12">
                                        <input id="email" type="email"
                                            class="form-control @error('email') is-invalid @enderror" name="email"
                                            value="{{ old('email') }}" required autocomplete="email" maxlength="{{config('setting.email_max_size')}}">
    
                                        @error('email')
                                            <span class="invalid-feedback" role="alert">
                                                <strong>{{ $message }}</strong>
                                            </span>
                                        @enderror
                                    </div>
                                </div>
    
                                <div class="form-group row">
                                    <label for="password"
                                        class="col-12 col-form-label">{{ __('Password') }}</label>
    
                                    <div class="col-12">
                                        <input id="password" type="password"
                                            class="form-control @error('password') is-invalid @enderror" name="password"
                                            required autocomplete="new-password" minlength="{{config('setting.password_min_size')}}">
    
                                        @error('password')
                                            <span class="invalid-feedback" role="alert">
                                                <strong>{{ $message }}</strong>
                                            </span>
                                        @enderror
                                    </div>
                                </div>
    
                                <div class="form-group row">
                                    <label for="password-confirm"
                                        class="col-12 col-form-label">{{ __('Confirm Password') }}</label>
    
                                    <div class="col-12">
                                        <input id="password-confirm" type="password" class="form-control"
                                            name="password_confirmation" required autocomplete="new-password"
                                            minlength="{{config('setting.password_min_size')}}">
                                    </div>
                                </div>
    
                                <div class="form-group row mb-0">
                                    <div class="col-12">
                                        <button type="submit" class="btn btn-primary">
                                            {{ __('Register') }}
                                        </button>
                                    </div>
                                </div>
                            </form>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>
@endsection
