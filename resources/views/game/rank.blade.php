@extends('layouts.app', [
    'title' => $post->title.' - '.__('title.rank'),
    'ogTitle' => $post->title.' - '.__('title.rank'),
    'ogImage' => $ogElement?->thumb_url,
    'ogDescription' => $post->description,
    'embed' => $embed,
])

@section('header')
    <script src="https://embed.twitch.tv/embed/v1.js"></script>
@endsection

@section('content')
    <Rank inline-template 
        comment-max-length="{{ config('setting.comment_max_length') }}"
        index-comment-endpoint="{{ route('api.public-post.comment.index', $post->serial) }}"
        create-comment-endpoint="{{ route('api.public-post.comment.create', $post->serial) }}"
        report-comment-endpoint="{{ route('api.public-post.comment.report', [$post->serial, '_comment_id']) }}"
        :champion-histories="{{ json_encode($champion_histories) }}"
        :max-rank="{{ $reports->total()}}"
        :game-statistic="{{ $gameResult ? json_encode($gameResult->statistics) : 'null' }}"
    >
        {{-- Main --}}
        <div class="container" v-cloak>
            @if(!$embed)
            <div class="row mb-3">
                <div class="col-auto">
                    <a class="btn btn-outline-dark btn-sm m-1" href="{{route('home')}}"><i class="fa-solid fa-home"></i>&nbsp;{{__('rank.return_home')}}</a>
                    <a class="btn btn-outline-dark btn-sm m-1" href={{route('game.show', $post->serial)}}><i class="fa-solid fa-play"></i>&nbsp;{{__('rank.play')}}</a>
                    <button @click="share" id="popover-button-event" type="button" class="btn btn-outline-dark btn-sm m-1"><i class="fa-solid fa-share-square"></i>&nbsp;{{__('rank.share')}}</button>
                    <b-popover ref="popover" target="popover-button-event" :disabled="true">{{__('Copied link')}}</b-popover>
                    @if($gameResult && !$shared)
                        <button @click="shareResult" id="share-result-button-event" type="button" class="btn btn-primary btn-sm m-1"><i class="fa-solid fa-share-square">&nbsp;{{__('rank.share-result')}}</i></button>
                        <b-popover ref="share-popover" target="share-result-button-event" :disabled="true">{{__('Copied link')}}</b-popover>
                    @endif
                </div>
            </div>
            <hr>
            
            <div class="d-flex position-relative">
                <h1 class="break-all post-title">{{ $post->title }} - {{__('Ranking')}}</h1>
                @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])
            </div>
            <p>{{ $post->description }}</p>
            @endif

            <b-tabs content-class="mt-3" nav-wrapper-class="@if($gameResult) sticky-top bg-default @endif">
                {{-- my game result --}}
                @if($gameResult)
                <b-tab :title="$t('My Rank')" {{request('tab') == 0 ? 'active':''}} @click="clickTab('0')">
                    <div class="card my-2 card-hover">
                        <div class="card-header rank-header">
                            <span class="text-left w-25 rank-number">1</span>
                            <h2 class="text-center d-none d-md-block w-50 element-title">{{ $gameResult->winner->title }}</h2>
                            <div class="text-right ml-auto">
                                {{ __('Global Rank') }}:&nbsp;{{ $gameResult->winner_rank ?? __('none') }}<br>
                            </div>
                        </div>
                        {{-- Rank #1 --}}
                        <div class="card-body text-center rank-card">
                            <h2 class="text-center d-block d-md-none element-title">{{ $gameResult->winner->title }}</h2>
                            @include('game.partial.my-champion-container', ['gameResult' => $gameResult])

                            
                            @if(config('setting.show_rank_history'))
                            <div class="custom-control custom-switch text-right">
                                <input type="checkbox" class="custom-control-input" id="switchRankHistory" v-model="showRankHistory">
                                <label class="custom-control-label btn-link" for="switchRankHistory"><i class="fa-solid fa-chart-line"></i>&nbsp;@{{$t('rank.chart.title.rank_history')}}</label>
                            </div>
                            @endif
                            <div class="custom-control custom-switch text-right">
                                <input type="checkbox" class="custom-control-input" id="switchTimeline" v-model="showMyTimeline">
                                <label class="custom-control-label btn-link" for="switchTimeline"><i class="fa-solid fa-chart-line"></i>&nbsp;@{{$t('rank.chart.title.timeline')}}</label>
                            </div>

                            <div class="row">
                                @if(config('setting.show_rank_history'))
                                <div v-show="showRankHistory" id="my-champion-container" class="col-12" :class="{'col-xl-6': showMyTimeline}" style="overflow-x: scroll;">
                                    <div class="rank-chart-container d-flex align-content-center justify-content-center p-0">
                                        <canvas id="my-champion"></canvas>
                                    </div>
                                </div>
                                @else
                                <div class="offset-3"></div>
                                @endif

                                <div v-show="showMyTimeline" id="my-timeline-container" class="overflow-scroll hide-scrollbar col-12" :class="{'col-xl-6': showRankHistory}">
                                    <div class="rank-chart-container d-flex align-content-center justify-content-center p-0" style="min-width: {{400 + $gameResult->rounds * 8}}px">
                                        <canvas id="my-timeline"></canvas>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    {{-- Rank #2 ~ #10 --}}
                    @foreach ($gameResult->data as $index => $rank)
                    
                    <div class="card my-2 card-hover">
                        <div class="card-header rank-header">
                            <span class="text-left w-25 rank-number">{{ (int)$index + 2 }}</span>
                            <h2 class="text-center d-none d-md-block w-50 element-title">{{ $rank->loser->title }}</h2>
                            <div class="text-right ml-auto w-auto">
                                {{ __('Global Rank') }}:&nbsp;{{ $rank->rank ?? __('none') }}
                            </div>
                        </div>
                        <div class="card-body text-center rank-card">
                            <h2 class="text-center d-block d-md-none element-title">{{ $rank->loser->title }}</h2>
                            @include('game.partial.my-element-container', ['rank' => $rank])
                        </div>
                    </div>

                    @if(config('services.google_ad.enabled') && config('services.google_ad.rank_page') && $index == 3)
                    <div class="row">
                        <div id="google-ad-1" class="col-12">
                            @include('ads.rank_ad_1', ['id' => 'google-ad-1'])
                        </div>
                    </div>
                    @endif
                    @endforeach
                </b-tab>
                @endif
                {{-- Global Rank --}}
                <b-tab :title="$t('Global Rank')" {{request('tab') == 1 ? 'active':''}} @click="clickTab('1')">
                    @foreach ($reports as $index => $rank)
                    <div class="card my-2 card-hover">
                        <div class="card-header rank-header">
                            <span class="text-left w-25 rank-number">{{ $rank->rank }}</span>
                            <h2 class="text-center d-none d-md-block w-50 element-title">{{ $rank->element->title }}</h2>
                            <div class="text-right ml-auto w-auto">
                                @if($rank->win_rate)
                                    {{ __('rank.win_rate') }}:&nbsp;{{ round($rank->win_rate,1) }}%
                                @else
                                    {{ __('rank.win_rate') }}:&nbsp;0%
                                @endif
                            </div>
                        </div>
                        <div class="card-body text-center rank-card">
                            <div>
                                <h2 class="text-center d-block d-md-none element-title">{{ $rank->element->title }}</h2>
                                @if($rank->rank === 1)
                                    <div class="col-12 align-content-center justify-content-center">
                                        @include('game.partial.global-element-container', ['rank' => $rank])
                                    </div>
                                    @if(config('setting.show_rank_history'))
                                    <div id="global-champion-container" class="col-12 overflow-scroll">
                                        <div class="rank-chart-container d-flex align-content-center justify-content-center">
                                            <canvas id="global-champion"></canvas>
                                        </div>
                                    </div>
                                    @endif
                                @else
                                    @include('game.partial.global-element-container', ['rank' => $rank])
                                @endif
                            </div>
                        </div>
                    </div>    

                    @if(config('services.google_ad.enabled') && config('services.google_ad.rank_page') && $index == 4)
                    <div class="row">
                        <div id="google-ad-2" class="col-12">
                            @include('ads.rank_ad_2',['id' => 'google-ad-2'])
                        </div>
                    </div>
                    @endif
                    @endforeach
                    
                    @if($reports && count($reports) == 0)
                    <div class="card my-2 card-hover">
                        <div class="card-body text-center rank-card">
                            <div class="align-self-center">
                                <small>{{ __('rank.no_data') }}</small>
                            </div>
                        </div>
                    </div>
                    @endif
                    {{-- Pagination --}}
                    <div class="d-none d-md-block">
                        <div class="row justify-content-center pt-2">
                            {{ $reports->onEachSide(2)->appends(request()->except('page'))->appends(['tab'=>1])->links() }}
                        </div>
                    </div>
                    <div class="d-block d-md-none">
                        <div class="d-flex justify-content-center pt-2">
                            @include('layouts.partials.mobile-pagination', ['paginator' => $reports])
                        </div>
                    </div>
                </b-tab>
            </b-tabs>

            {{-- Comment --}}
            <hr class="my-4">
            <h5 id="comments-total" class="d-inline">{{ __('Comment') }}(@{{meta.total}})</h5>
            <div class="card mb-4">
                {{-- Comments --}}
                <div class="card-body">
                    <div v-for="comment in comments">
                        <div class="d-flex justify-content-between w-100">
                            {{-- avatar --}}
                            <div class="avatar-container">
                                <div class="avatar">
                                    <img v-if="comment.avatar_url" :src="comment.avatar_url" :alt="comment.nickname">
                                    <img v-else src="{{asset('storage/default-avatar.webp')}}" :alt="comment.nickname">
                                </div>
                            </div>
                            <div class="comment-container">
                                {{--nickname--}}
                                <div class="d-flex justify-content-between">
                                    <span class="text-black-50 font-size-large" style="overflow-wrap:anywhere"><h5>@{{ comment.nickname }}</h5></span>
                                    <div class="ml-auto">
                                        <div class="text-align-end">
                                            <div class="d-flex">
                                                {{-- timestamp --}}
                                                <div class="text-right text-muted mr-4">
                                                    @{{comment.created_at | formNow}}
                                                </div>  
                                                {{-- options --}}
                                                <div class="dropdown">
                                                    <span href="#" role="button" id="reportDropdown" data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
                                                        <i class="fa-xl fa-solid fa-ellipsis-vertical cursor-pointer text-center" style="width: 20px"></i>
                                                    </span>
                                                        
                                                    <div class="dropdown-menu dropdown-menu-right" aria-labelledby="reportDropdown">
                                                        <a class="dropdown-item" @click.prevent="reportComment(comment)" href="#"><i class="fa-solid fa-triangle-exclamation"></i>&nbsp;{{__('comment.report')}}</a>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                {{-- champions --}}
                                <h5 class="overflow-hidden" v-if="comment.champions.length > 0">
                                    <template v-for="champion in comment.champions">
                                        <h5 class="badge badge-secondary mr-1 mb-1 rounded-0" :title="champion">@{{champion}}</h5>
                                    </template>
                                </h5>
                                {{-- comment --}}
                                <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar" style="max-height: 200px;">@{{comment.content}}</p>
                            </div>
                        </div>

                        {{-- display for mobile (width < 576px) --}}
                        <div class="d-block d-sm-none">
                            {{-- avatar --}}
                            <div class="d-flex">
                                <div class="" style="margin-right:20px">
                                    <div class="avatar">
                                        <img v-if="comment.avatar_url" :src="comment.avatar_url" :alt="comment.nickname">
                                    <img v-else src="{{asset('storage/default-avatar.webp')}}" :alt="comment.nickname">
                                    </div>
                                </div>
                                
                                <div>
                                    {{--nickname--}}
                                    <span class="text-black-50 font-size-large text-break"><h5> @{{ comment.nickname }}</h5></span>
                                    {{-- timestamp --}}
                                    <div class="text-muted">
                                        @{{comment.created_at | formNow}}
                                    </div>  
                                </div>
                                <div class="ml-auto">
                                    <div class="text-align-end">
                                        <div class="dropdown">
                                            <span href="#" role="button" id="reportDropdown" data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
                                                <i class="fa-xl fa-solid fa-ellipsis-vertical cursor-pointer text-center" style="width: 20px"></i>
                                            </span>

                                            <div class="dropdown-menu dropdown-menu-right" aria-labelledby="reportDropdown">
                                                <a class="dropdown-item" href="#" @click.prevent="reportComment(comment)"><i class="fa-solid fa-triangle-exclamation"></i>&nbsp;{{__('comment.report')}}</a>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                                
                            {{-- champions --}}
                            <h5 class="overflow-hidden" v-if="comment.champions.length">
                                <template v-for="champion in comment.champions">
                                    <h5 class="badge badge-secondary mr-1 mb-1 rounded-0" :title="champion">@{{champion}}</h5>
                                </template>
                            </h5>

                            {{-- comment --}}
                            <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar" style="max-height: 200px;">@{{comment.content}}</p>
                        </div>
                        <hr class="my-1">
                    </div>
                    {{-- Pagination --}}
                    <b-pagination
                        v-if="meta.last_page > 1"
                        v-model="meta.current_page"
                        :total-rows="meta.total"
                        :per-page="meta.per_page"
                        @input="changePage"
                        class="justify-content-center"
                    ></b-pagination>
                </div>

                {{-- leave a comment --}}
                <div class="card-body">
                    <div class="row">
                        {{-- comment input --}}
                        <div class="col-12">
                            <div class="">
                            {{__('Nickname')}}: <span class="text-black-50 font-size-large text-break">
                                <h5 class="d-inline" v-if="!anonymous">@{{profile.nickname}}</h5>
                                <h5 class="d-inline" v-if="anonymous">****</h5>
                            </span>
                                {{-- anonymous mode --}}
                                <div class="mr-auto from-check" v-if="profile.is_auth">
                                    <input class="from-check" type="checkbox" id="anonymous" v-model="anonymous">
                                    <label class="from-control" for="anonymous">{{__('comment.anonymous')}}</label>
                                </div>
                            </div>
                            <div class="overflow-hidden" v-if="profile.champions.length > 0">
                                {{__('Vote result')}}:
                                {{-- champions --}}
                                <h5 class="d-inline">
                                    <template v-for="champion in profile.champions">
                                        <span class="badge badge-secondary mr-1 mb-1" :title="champion">@{{champion}}</span>
                                    </template>
                                </h5>
                            </div>
                        </div>
                        {{-- words --}}
                        <div class="col-12">
                            <div class="form-group">
                                <textarea class="form-control comment-input-field" rows="3" maxlength="{{config('setting.comment_max_length')}}" placeholder="{{__('comment.leave_comment')}}" style="resize: none" v-model="commentInput"></textarea>
                                <span> (@{{commentWords}}/@{{commentMaxLength}})</span>
                            </div>
                        </div>
                        {{-- submit --}}
                        <div class="col-12">
                            <button type="button" class="btn btn-primary" :disabled="!validComment || isSubmiting" @click="submitComment">{{__('comment.submit')}}</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </rank>
@endsection
