@extends('layouts.app', ['title' => __('My Votes')])

@section('header')
    <meta name="robots" content="noindex" />
@endsection

@section('content')
    <div class="container-fluid">
        <div class="row">
            <div class="d-none d-md-block col-md-2">
                @include('account.profile.partial.tabs', ['active' => 'profile'])
            </div>
            <profile 
                inline-template 
                nickname-max-length="{{ config('setting.user_name_max_size') }}"
                props-nickname="{{ auth()->user()->name }}" 
                props-email="{{ auth()->user()->email }}"
                props-avatar-url="{{ auth()->user()->avatar_url ?? asset('storage/default-avatar.webp') }}"
            >
                <div class="col-xs-8 col-md-10" v-cloak>
                    <div class="card mr-4 mb-4">
                        <div class="card-header">{{ __('Profile') }}</div>
                        <div class="card-body">
                            <div class="row">
                                <div class="col-md-6 col-sm-12">
                                    <form method="POST" action="{{ route('profile.update') }}"
                                        enctype="multipart/form-data">
                                        @csrf
                                        @method('put')
                                        <div class="form-group">
                                            <h5><label for="name">{{ __('Avatar') }}</label></h5>
                                            <div class="d-flex w-100">
                                                <div class="w-25">
                                                    <div class="avatar">
                                                        <img :src="avatarUrl" class="cursor-pointer"
                                                            alt="{{ __('Avatar') }}" @click="uploadAvatar">
                                                        <input type="file" id="avatar-upload" class=" d-none"
                                                            name="avatar" @change="handleAvatarChange">
                                                    </div>
                                                </div>
                                                <div class="w-auto pl-3 d-none d-sm-block">
                                                    <img :src="avatarUrl" class="cursor-pointer w-100"
                                                        style="max-height: 300px" alt="{{ __('Avatar') }}"
                                                        @click="uploadAvatar">
                                                </div>
                                            </div>

                                            <button type="submit" class="btn btn-primary btn-block m-1"
                                                :disabled="!isAvatarChanged">{{ __('Update Avatar') }}</button>
                                        </div>

                                    </form>
                                </div>


                                <div class="col-md-6 col-sm-12">
                                    <hr class="d-block d-md-none">
                                    <form method="POST" action="{{ route('profile.update') }}">
                                        @csrf
                                        @method('put')
                                        <div class="form-group">
                                            <h5><label for="name">{{ __('Nickname') }}</label></h5>
                                            <input type="text" class="form-control @error('name') is-invalid @enderror"
                                                autocomplete="off" id="name" minlength="1"
                                                maxlength="{{ config('setting.user_name_max_size') }}" name="name"
                                                v-model="nickname">
                                            @error('name')
                                                <div class="invalid-feedback">{{ $message }}</div>
                                            @enderror
                                            <small id="nameHelp"
                                                class="form-text text-muted">{{ __('nickname_update_rate') }}&nbsp;(@{{ nicknameLength }}
                                                / @{{ nicknameMaxLength }})</small>

                                            <button type="submit" class="btn btn-primary btn-block  m-1 p-1"
                                                :disabled="!isNicknameChanged">{{ __('Update Nickname') }}</button>
                                        </div>
                                    </form>
                                </div>


                                <div class="col-md-6 col-sm-12">
                                    <hr class="d-block d-md-none">
                                    <div class="form-group">
                                        <h5>
                                            <label for="email">{{ __('Email') }}
                                                <i v-show="!maskEmail" class="fa-solid fa-eye-slash cursor-pointer"
                                                    @click="toggleMaskEmail"></i>
                                                <i v-show="maskEmail" class="fa-solid fa-eye cursor-pointer"
                                                    @click="toggleMaskEmail"></i>
                                            </label>
                                        </h5>

                                        <input v-show="maskEmail" class="form-control" disabled ="text" name="maskedEmail"
                                            :value="maskedEmail">
                                        <input v-show="!maskEmail" type="text"
                                            class="form-control @error('email') is-invalid @enderror" disabled
                                            autocomplete="off" id="email"
                                            maxlength="{{ config('setting.email_max_size') }}" name="email"
                                            v-model="email">
                                        @error('email')
                                            <div class="invalid-feedback">{{ $message }}</div>
                                        @enderror
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="card mr-4 mb-4">
                        <div class="card-header">{{ __('Change Password') }}</div>
                        <div class="card-body">
                            <div class="row">
                                <div class="col-md-6 col-sm-12">
                                    @if (Auth::user()->password === '')
                                        <form method="POST" action="{{ route('profile.update.password.init') }}">
                                            @csrf
                                            @method('put')
                                            <div class="form-group">
                                                <h5><label for="new_password">{{ __('New Password') }}</label></h5>
                                                <input type="password"
                                                    class="form-control @error('new_password') is-invalid @enderror"
                                                    id="new_password" name="new_password" required>
                                                @error('new_password')
                                                    <div class="invalid-feedback">{{ $message }}</div>
                                                @enderror
                                            </div>
                                            <div class="form-group">
                                                <h5><label
                                                        for="new_password_confirmation">{{ __('Confirm New Password') }}</label>
                                                </h5>
                                                <input type="password" class="form-control" id="new_password_confirmation"
                                                    name="new_password_confirmation" required>
                                            </div>
                                            <button type="submit"
                                                class="btn btn-primary btn-block">{{ __('Update Password') }}</button>
                                        </form>
                                    @else
                                        <form method="POST" action="{{ route('profile.update.password') }}">
                                            @csrf
                                            @method('put')
                                            <div class="form-group">
                                                <h5><label for="current_password">{{ __('Current Password') }}</label>
                                                </h5>
                                                <input type="password"
                                                    class="form-control @error('current_password') is-invalid @enderror"
                                                    id="current_password" name="current_password" required>
                                                @error('current_password')
                                                    <div class="invalid-feedback">{{ $message }}</div>
                                                @enderror
                                            </div>
                                            <div class="form-group">
                                                <h5><label for="new_password">{{ __('New Password') }}</label></h5>
                                                <input type="password"
                                                    class="form-control @error('new_password') is-invalid @enderror"
                                                    id="new_password" name="new_password" required>
                                                @error('new_password')
                                                    <div class="invalid-feedback">{{ $message }}</div>
                                                @enderror
                                            </div>
                                            <div class="form-group">
                                                <h5><label
                                                        for="new_password_confirmation">{{ __('Confirm New Password') }}</label>
                                                </h5>
                                                <input type="password" class="form-control"
                                                    id="new_password_confirmation" name="new_password_confirmation"
                                                    required>
                                            </div>
                                            <button type="submit"
                                                class="btn btn-primary btn-block">{{ __('Update Password') }}</button>
                                        </form>
                                    @endif
                                </div>
                            </div>
                        </div>
                    </div>
                    @if(!app()->isProduction())
                    <div class="card mr-4 mb-4">
                        <div class="card-header">{{ __('Social Media Accounts') }} ({{ __('Quick Login') }})</div>
                        <div class="card-body">
                            <div class="row">
                                <div class="col-md-6 col-sm-12">
                                    <span class="social-logo-wrapper"><img class="social-logo"
                                            src="{{ asset('storage/google-logo.svg') }}" alt="Google logo"></span>
                                    @if (Auth::user()->user_socialite?->google_id)
                                        <b class="text-uppercase">
                                            {{ __('Connected') }}&nbsp;<i class="fa-solid fa-circle-check text-success"></i> 
                                        </b>
                                        ({{mask_email(Auth::user()->user_socialite->google_email,2,2)}})
                                    @else
                                        <a class="d-inline btn btn-outline-light btn-block text-dark border-secondary"
                                            href="{{ route('auth.connect.google') }}">
                                            <span class="">{{ __('Connect to Google') }}</span>
                                        </a>
                                    @endif
                                </div>
                            </div>
                        </div>
                    </div>
                    @endif
                </div>
            </profile>
        </div>
    </div>
    </profile>
@endsection

@section('footer')
    @include('partial.footer')
@endsection
