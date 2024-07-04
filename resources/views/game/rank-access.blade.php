@extends('layouts.app', [
    'title' => __('title.access').' - '.__('title.rank'),
    'ogTitle' => __('title.access').' - '.__('title.rank'),
])

@section('header')
    <script src="https://embed.twitch.tv/embed/v1.js"></script>
@endsection

@section('content')
    {{-- Main --}}
    <rank-access
        inline-template
        post-serial="{{ $serial }}"
        access-endpoint={{ route('api.game.access', $serial) }}
        redirect-url="{{ route('game.rank', $serial) }}"
    >
        <div class="container" v-cloak>
            <div id="modal" class="modal" data-backdrop="static" data-keyboard="false" tabindex="-1"
                aria-labelledby="gameSettingPanelLabel" aria-hidden="true">
                <div class="modal-dialog">
                    <div class="modal-content">

                        <div class="modal-header">
                            <h5 class="modal-title align-self-center" id="gameSettingPanelLabel">{{ __('title.access')   }}</h5>
                            <div>
                                <a class="btn btn-outline-secondary" href="/">
                                    <i class="fas fa-times"></i>&nbsp;@{{ $t('game.cancel') }}
                                </a>
                            </div>
                        </div>

                        <div class="modal-body">
                        {{-- invalid password text --}}
                        <div class="alert alert-danger" v-if="isInvalidPassword">
                            @{{ $t('game.invalid_password') }}
                        </div>

                        {{-- password required --}}
                        <div class="card">
                            <div class="input-group mb-3">
                                <label class="input-group-text" for="inputPassword">@{{ $t('game.password') }}</label>
                                <input type="text" class="form-control font-size-16" v-model="inputPassword" autocomplete="off">
                                <div class="input-group-append">
                                  <button class="btn btn-primary" @click="submitPassword" :disabled="inputPassword.length == 0">
                                    <i class="fas fa-key"></i>&nbsp;@{{ $t('Enter') }}
                                  </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </rank-access>

@endsection
