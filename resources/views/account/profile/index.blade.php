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
            <profile inline-template nickname-max-length="{{ config('setting.user_name_max_size') }}"
                props-nickname="{{ auth()->user()->name }}" props-email="{{ auth()->user()->email }}"
                props-avatar-url="{{ auth()->user()->avatar_url ?? asset('storage/default-avatar.webp') }}">
                <div class="col-xs-8 col-md-10" v-cloak>
                    <div class="card mr-4 mb-4">
                        <div class="card-header">{{ __('Profile') }}</div>
                        <div class="card-body">
                            <div class="row">
                                <div class="col-6">
                                    <form method="POST" action="{{ route('profile.update') }}"
                                        enctype="multipart/form-data">
                                        @csrf
                                        @method('put')
                                        <div class="form-group">
                                            <label for="name">{{ __('Avatar') }}</label>
                                            <div class="avatar">
                                                <img :src="avatarUrl" class="cursor-pointer" alt="{{ __('Avatar') }}"
                                                    @click="uploadAvatar">
                                                <input type="file" id="avatar-upload" class=" d-none" name="avatar"
                                                    @change="handleAvatarChange">
                                            </div>
                                            <button type="submit" class="btn btn-primary"
                                                :disabled="!isAvatarChanged">{{ __('Update Avatar') }}</button>
                                        </div>

                                    </form>
                                </div>
                                <div class="col-6">
                                    <form method="POST" action="{{ route('profile.update') }}">
                                        @csrf
                                        @method('put')
                                        <div class="form-group">
                                            <label for="name">{{ __('Nickname') }}</label>
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

                                            <button type="submit" class="btn btn-primary"
                                                :disabled="!isNicknameChanged">{{ __('Update Nickname') }}</button>
                                        </div>
                                    </form>
                                </div>
                                <div class="col-6">
                                    <div class="form-group">
                                        <label for="email">{{ __('Email') }}</label>
                                        <i v-show="!maskEmail" class="fa-solid fa-eye-slash cursor-pointer"
                                            @click="toggleMaskEmail"></i>
                                        <i v-show="maskEmail" class="fa-solid fa-eye cursor-pointer"
                                            @click="toggleMaskEmail"></i>
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
                            <div class="col-6">
                                <form method="POST" action="{{route('profile.update.password')}}">
                                    @csrf
                                    @method('put')
                                    <div class="form-group">
                                        <label for="current_password">{{ __('Current Password') }}</label>
                                        <input type="password"
                                            class="form-control @error('current_password') is-invalid @enderror"
                                            id="current_password" name="current_password" required>
                                        @error('current_password')
                                            <div class="invalid-feedback">{{ $message }}</div>
                                        @enderror
                                    </div>
                                    <div class="form-group">
                                        <label for="new_password">{{ __('New Password') }}</label>
                                        <input type="password"
                                            class="form-control @error('new_password') is-invalid @enderror"
                                            id="new_password" name="new_password" required>
                                        @error('new_password')
                                            <div class="invalid-feedback">{{ $message }}</div>
                                        @enderror
                                    </div>
                                    <div class="form-group">
                                        <label for="new_password_confirmation">{{ __('Confirm New Password') }}</label>
                                        <input type="password" class="form-control" id="new_password_confirmation"
                                            name="new_password_confirmation" required>
                                    </div>
                                    <button type="submit" class="btn btn-primary">{{ __('Update Password') }}</button>
                                </form>
                            </div>
                        </div>

                    </div>
            </profile>
        </div>
    </div>
    </profile>
@endsection

@section('footer')
    @include('partial.footer')
@endsection
