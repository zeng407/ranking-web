@extends('layouts.app', [
    'title' => $post,
    'ogTitle' => $post->title,
    'ogImage' => $element->thumb_url,
    'ogDescription' => $post->description,
])

@section('content')
    <rank inline-template>
        <!--  Main -->
        <div class="container" v-cloak>
            <div>
                <h2>{{ $post->title }}</h2>
                <p>{{ $post->description }}</p>
            </div>
            <b-tabs content-class="mt-3">
                @if($gameResult)
                    <b-tab :title="$t('My Rank')" {{request('tab') == 0 ? 'active':''}} @click="clickTab('0')">
                        <div class="card my-2 card-hover">
                            <div class="card-header rank-header">
                                <h2 class="text-left">1</h2>
                                <div class="text-center d-none d-md-block">{{ $gameResult->winner->title }}</div>
                                <div class="text-right">
                                    {{ __('Global Rank') }}:&nbsp;{{ $gameResult->winner_rank ?? __('none') }}
                                </div>
                            </div>
                            {{-- Rank #1 --}}
                            <div class="card-body text-center rank-card">
                                <div class="text-center d-block d-md-none">{{ $gameResult->winner->title }}</div>
                                @if($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'youtube')
                                    <youtube-player 
                                        width="100%" height="420" ref-id="{{ $gameResult->winner->id }}"
                                        video-id="{{ $gameResult->winner->video_id }}"
                                        :controls="true" 
                                        :autoplay="false" 
                                        :rel="0" origin="{{ request()->getSchemeAndHttpHost() }}">
                                    </youtube-player>
                                @elseif($gameResult->winner->type === 'video')
                                    <video width="100%" height="270" loop autoplay muted playsinline
                                        src="{{ $gameResult->winner->thumb_url }}"></video>
                                @elseif($gameResult->winner->type === 'image')
                                    <img src="{{ $gameResult->winner->thumb_url }}" height="270" class="w-100"
                                        alt="{{ $gameResult->winner->title }}">
                                @endif
                            </div>
                        </div>
                        {{-- Rank #2 ~ #10 --}}
                        <div class="row">
                        @foreach ($gameResult->data as $index => $rank)
                            <div class="col-md-12">
                                <div class="card my-2 card-hover">
                                    <div class="card-header rank-header">
                                        <h2 class="text-left">{{ (int)$index + 2 }}</h2>
                                        <div class="text-center d-none d-md-block">{{ $rank->loser->title }}</div>
                                        <div class="text-right">
                                            {{ __('Global Rank') }}:&nbsp;{{ $rank->rank ?? __('none') }}
                                        </div>
                                    </div>
                                    <div class="card-body text-center rank-card">
                                        <div class="text-center d-block d-md-none">{{ $rank->loser->title }}</div>
                                        @if($rank->loser->type === 'video' && $rank->loser->video_source === 'youtube')
                                            <youtube-player 
                                                width="100%" height="270" ref-id="{{ $rank->loser->id }}"
                                                video-id="{{ $rank->loser->video_id }}"
                                                :controls="true"
                                                :autoplay="false" 
                                                :rel="0" 
                                                origin="{{ request()->getSchemeAndHttpHost() }}">
                                        @elseif($rank->loser->type === 'video')
                                            <video width="100%" height="270" loop autoplay muted playsinline src="{{$rank->loser->thumb_url}}"></video>    
                                        @elseif($rank->loser->type === 'image')
                                            <img src="{{$rank->loser->thumb_url}}" height="270" class="w-100" alt="{{$rank->loser->title}}">
                                        @endif
                                    </div>
                                </div>
                            </div>
                        @endforeach
                        </div>
                    </b-tab>
                @endif
                {{-- Global Rank --}}
                <b-tab :title="$t('Global Rank')" {{request('tab') == 1 ? 'active':''}} @click="clickTab('1')">
                    @foreach ($reports as $index => $rank)
                        <div class="card my-2 card-hover">
                            <div class="card-header rank-header">
                                <h2 class="text-left">{{ $rank->rank }}</h2>
                                <div class="text-center d-none d-md-block">{{ $rank->element->title }}</div>
                                <div class="text-right">
                                    @if($rank->final_win_rate)
                                        {{ __('rank.win_at_final') }}:&nbsp;{{ $rank->final_win_rate }}%<br>
                                    @endif
                                    @if($rank->win_rate)
                                        {{ __('rank.win_rate') }}:&nbsp;{{ $rank->win_rate }}%
                                    @else
                                        {{ __('rank.win_rate') }}:&nbsp;0%
                                    @endif

                                </div>
                            </div>
                            <div class="card-body text-center rank-card">
                                <div class="text-center d-block d-md-none">{{ $rank->element->title }}</div>
                                @if($rank->element->type === 'video' && $rank->element->video_source === 'youtube')
                                    <youtube-player 
                                        width="100%" height="270" ref-id="{{ $rank->element->id }}"
                                        video-id="{{ $rank->element->video_id }}"
                                        :controls="true"
                                        :autoplay="false" 
                                        :rel="0" 
                                        origin="{{ request()->getSchemeAndHttpHost() }}">
                                    </youtube-player>
                                @elseif($rank->element->type === 'video')
                                    <video width="100%" height="270" loop autoplay muted playsinline src="{{$rank->element->thumb_url}}"></video>
                                @elseif($rank->element->type === 'image')
                                    <img src="{{$rank->element->thumb_url}}" height="270" class="w-100" alt="{{$rank->element->title}}">
                                @endif
                            </div>
                        </div>    
                    @endforeach
                    
                    @if($reports && count($reports) == 0)
                        <div class="card my-2 card-hover">
                            <div class="card-body text-center rank-card">
                                <div class="align-self-center">
                                    {{ __('rank.no_data') }}
                                </div>
                            </div>
                        </div>
                    @endif

                    <div class="row justify-content-center pt-2">
                        {{ $reports->appends(request()->except('page'))->appends(['tab'=>1])->links() }}
                    </div>
                </b-tab>
            </b-tabs>
        </div>
    </rank>
@endsection
