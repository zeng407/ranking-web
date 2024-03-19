@extends('layouts.app', [
    'title' => $post->title.' | '.__('title.rank'),
    'ogTitle' => $post->title.' | '.__('title.rank'),
    'ogImage' => $ogElement?->thumb_url,
    'ogDescription' => $post->description,
])

@section('content')
    <rank inline-template 
        comment-max-length="{{ config('setting.comment_max_length') }}"
        index-comment-endpoint="{{ route('api.public-post.comment.index', $post->serial) }}"
        create-comment-endpoint="{{ route('api.public-post.comment.create', $post->serial) }}"
        report-comment-endpoint="{{ route('api.public-post.comment.report', [$post->serial, '_comment_id']) }}"
    >
        {{-- Main --}}
        <div class="container" v-cloak>
            <div>
                <h2>{{ $post->title }}</h2>
                <p>{{ $post->description }}</p>
            </div>
            <b-tabs content-class="mt-3" nav-wrapper-class="@if($gameResult) sticky-top bg-default @endif">
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
                                    width="100%" height="270" ref-id="{{ $gameResult->winner->id }}"
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
                    @foreach ($gameResult->data as $index => $rank)
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
                    @endforeach
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
                                    {{ __('rank.win_at_final') }}:&nbsp;{{ round($rank->final_win_rate,1) }}%<br>
                                @endif
                                @if($rank->win_rate)
                                    {{ __('rank.win_rate') }}:&nbsp;{{ round($rank->win_rate,1) }}%
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
                    {{-- Pagination --}}
                    <div class="row justify-content-center pt-2">
                        {{ $reports->appends(request()->except('page'))->appends(['tab'=>1])->links() }}
                    </div>
                </b-tab>
            </b-tabs>

            {{-- Comment --}}
            <hr class="my-4">
            <h5>{{ __('Comment') }}(@{{meta.total}})</h5>
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
                                    <span class="text-black-50 font-size-large" style="overflow-wrap:anywhere"><small>@{{ comment.nickname }}</small></span>
                                    <div class="ml-auto">
                                        <div class="text-align-end">
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
                                {{-- champions --}}
                                <div class="overflow-hidden">
                                    <template v-for="champion in comment.champions">
                                        <span class="badge badge-secondary mr-1 mb-1" :title="champion">@{{champion}}</span>
                                    </template>
                                </div>
                                {{-- comment --}}
                                <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar" style="max-height: 200px;">@{{comment.content}}</p>
                                {{-- timestamp --}}
                                <div class="text-right text-muted">
                                    @{{comment.created_at | datetime}}
                                </div>  
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
                                    <span class="text-black-50 font-size-large text-break"><small> @{{ comment.nickname }}</small></span>
                                    {{-- timestamp --}}
                                    <div class="text-muted">
                                        @{{comment.created_at | datetime}}
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
                            <div class="overflow-hidden">
                                <template v-for="champion in comment.champions">
                                    <span class="badge badge-secondary mr-1 mb-1" :title="champion">@{{champion}}</span>
                                </template>
                            </div>

                            {{-- comment --}}
                            <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar" style="max-height: 200px;">@{{comment.content}}</p>
                        </div>
                        <hr class="my-4">
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
                            {{__('Nickname')}}: <span class="text-black-50 font-size-large text-break"><small>@{{profile.nickname}}</small></span>
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
                                <textarea class="form-control" rows="3" maxlength="{{config('setting.comment_max_length')}}" placeholder="{{__('comment.leave_comment')}}" style="resize: none" v-model="commentInput"></textarea>
                                <span> (@{{commentWords}}/@{{commentMaxLength}})</span>
                            </div>
                        </div>
                        {{-- submit --}}
                        <div class="col-12">
                            <button type="button" class="btn btn-primary" :disabled="!validComment" @click="submitComment">{{__('comment.submit')}}</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </rank>
@endsection
