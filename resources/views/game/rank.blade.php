@extends('layouts.app', [
    'title' => $post->title . ' - ' . __('title.rank'),
    'ogTitle' => $post->title . ' - ' . __('title.rank'),
    'ogImage' => $ogElement?->getLowThumbUrl(),
    'ogDescription' => $post->description,
    'embed' => $embed,
    'stickyNav' => 'sticky-top-desktop'
])

@section('header')
    <script src="https://embed.twitch.tv/embed/v1.js"></script>
@endsection

@section('content')
    <Rank inline-template post-serial="{{ $post->serial }}" comment-max-length="{{ config('setting.comment_max_length') }}"
        index-comment-endpoint="{{ route('api.public-post.comment.index', $post->serial) }}"
        create-comment-endpoint="{{ route('api.public-post.comment.create', $post->serial) }}"
        report-comment-endpoint="{{ route('api.public-post.comment.report', [$post->serial, '_comment_id']) }}"
        :champion-histories="{{ json_encode($champion_histories) }}" :max-rank="{{ $reports->total() }}"
        :game-statistic="{{ $gameResult ? json_encode($gameResult->statistics) : 'null' }}"
        :game-room-ranks="{{ $gameResult ? json_encode($gameResult->game_room) : 'null' }}">
        {{-- Main --}}
        <div class="container-fuild" v-cloak>
            <div class="row m-0">
                {{-- left part: ads --}}
                <div class="d-none d-lg-block col-lg-2">
                  @if(config('services.google_ad.enabled') && config('services.google_ad.rank_page'))
                  <div class="p-lg-2 p-xl-3">
                    @include('ads.rank_ad_sides')
                  </div>
                  @endif
                </div>

                {{-- main part --}}
                <div class="col-12 col-lg-8">
                    @if (!$embed)
                        <div class="row mb-3">
                            <div class="col-10">
                                <a class="btn btn-outline-dark btn-sm m-1" href="{{ route('home') }}">
                                  <h5 class="m-0"><i class="fa-solid fa-home"></i>&nbsp;{{ __('rank.return_home') }}</h5>
                                </a>
                                <a class="btn btn-outline-dark btn-sm m-1" href={{ route('game.show', $post->serial) }}>
                                  <h5 class="m-0"><i class="fa-solid fa-play"></i>&nbsp;{{ __('rank.play') }}</h5>
                                </a>
                                <div class="m-1 d-inline-block">
                                  <h5 class="m-0">
                                    <share-link heading-tag="h5" id="rank" :url="shareRankLink()" text="{{ __('rank.share') }}"
                                        after-copy-text="{{ __('Copied link') }}"></share-link>
                                  </h5>
                                </div>
                                @if ($gameResult && !$shared)
                                    <div class="m-1 d-inline-block">
                                      <h5 class="m-0">
                                        <share-link heading-tag="h5" id="result" :url="shareResultLink()"
                                            text="{{ __('rank.share-result') }}" after-copy-text="{{ __('Copied link') }}"
                                            custom-class="btn btn-primary btn-sm"></share-link>
                                      </h5>
                                    </div>
                                @endif
                            </div>
                            <div class="col-2">
                              @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])
                            </div>
                        </div>
                        <hr>

                        <div class="d-flex position-relative">
                            <h1 class="break-all post-title">{{ $post->title }} - {{ __('Ranking') }}</h1>
                        </div>
                        <p>{{ $post->description }}</p>
                    @endif

                    <b-tabs content-class="mt-3"
                        nav-wrapper-class="@if ($gameResult) sticky-top-rank-tab bg-default @endif rank-tabs hide-scrollbar">
                        {{-- my game result --}}
                        @if ($gameResult)
                            <b-tab {{ request('tab') == 0 ? 'active' : '' }} @click="clickTab('0')">
                              <template #title>
                                <h5><i class="fa-solid fa-trophy"></i>&nbsp;@{{$t('My Rank')}}</h5>
                              </template>
                                <div class="card my-2 card-hover">
                                  <div class="card-header rank-header">
                                    <span class="text-left w-25 rank-number">1</span>
                                    <h2 class="text-center d-none d-md-block w-50 element-title">
                                        {{ $gameResult->winner->title }}</h2>
                                    <div class="text-right ml-auto">
                                        {{ __('Global Rank') }}:&nbsp;{{ $gameResult->winner_rank ?? __('none') }}<br>
                                    </div>
                                  </div>
                                  {{-- Rank #1 --}}
                                  <div class="card-body text-center rank-card">
                                    <h2 class="text-center d-block d-md-none element-title ">
                                        {{ $gameResult->winner->title }}</h2>
                                    @include('game.partial.my-champion-container', [
                                        'gameResult' => $gameResult,
                                    ])
                                    <div class="custom-control custom-switch text-right">
                                        <input type="checkbox" class="custom-control-input" id="switchRankHistory"
                                            v-model="showRankHistory">
                                        <label class="custom-control-label btn-link" for="switchRankHistory"><i
                                                class="fa-solid fa-chart-line"></i>&nbsp;@{{ $t('rank.chart.title.rank_history') }}</label>
                                    </div>
                                    <div class="custom-control custom-switch text-right">
                                        <input type="checkbox" class="custom-control-input" id="switchTimeline"
                                            v-model="showMyTimeline">
                                        <label class="custom-control-label btn-link" for="switchTimeline"><i
                                                class="fa-solid fa-chart-line"></i>&nbsp;@{{ $t('rank.chart.title.timeline') }}</label>
                                    </div>

                                    <div class="row">
                                        <div v-show="showRankHistory" class="col-12"
                                            :class="{ 'col-xl-6': showMyTimeline }">
                                            <rank-history-chart
                                                chart-id="{{ 'my-rank-history-chart-' . $gameResult->winner->id }}"
                                                element-id="{{ $gameResult->winner->id }}"
                                                post-serial="{{ $post->serial }}"
                                                index-rank-endpoint="{{ route('api.rank.index') }}">
                                            </rank-history-chart>
                                        </div>

                                        <div v-show="showMyTimeline" id="my-timeline-container"
                                            class="col-12 hide-scrollbar-md overflow-x-scroll"
                                            :class="{ 'col-xl-6': showRankHistory }">
                                            <div class="rank-chart-container d-flex align-content-center justify-content-center p-0"
                                                style="min-width: {{ 400 + $gameResult->rounds * 8 }}px">
                                                <canvas id="my-timeline"></canvas>
                                            </div>
                                        </div>
                                        <div v-show="showMyTimeline" class="w-100">
                                            <div id="chartjs-tooltip"
                                                style="padding-left:15px; padding-top:15px; padding-right:15px; display:none">
                                                <div class="chart-tooltip-bg">
                                                    <table class="text-left">
                                                    </table>
                                                </div>
                                            </div>
                                            <div style="padding-left:15px; padding-top:15px; padding-right:15px">
                                                <div class="chart-tooltip-bg">
                                                    <table class="text-left">
                                                        <thead>
                                                            <tr style="border-width: 0px;">
                                                                <th style="border-width: 0px;">
                                                                    @{{ $t('rank.chart.total_time') }}:&nbsp@{{ secondsToHms(gameStatistic.game_time) }}
                                                                </th>
                                                            </tr>
                                                            <tr style="border-width: 0px;">
                                                                <th style="border-width: 0px;">
                                                                    @{{ $t('rank.chart.median_time') }}:&nbsp;@{{ computeMedianTime() }}
                                                                </th>
                                                            </tr>
                                                            <tr style="border-width: 0px;">
                                                                <th style="border-width: 0px;">
                                                                    @{{ $t('rank.chart.average_time') }}:&nbsp;@{{ computeAverageTime() }}
                                                                </th>
                                                            </tr>
                                                        </thead>
                                                    </table>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                  </div>
                                </div>
                                {{-- Rank #2 ~ #10 --}}
                                <div class="row">
                                @foreach ($gameResult->data as $index => $rank)
                                  <div class="{{$index < 3 ? 'col-12' : 'col-12 col-xl-6'}}">
                                    <div class="card my-2 card-hover">
                                        <div class="card-header rank-header">
                                            <span class="text-left w-25 rank-number">{{ (int) $index + 2 }}</span>
                                            <h2 class="text-center d-none d-md-block w-50 element-title">
                                                {{ $rank->loser->title }}</h2>
                                            <div class="text-right ml-auto w-auto">
                                                {{ __('Global Rank') }}:&nbsp;{{ $rank->rank ?? __('none') }}
                                            </div>
                                        </div>
                                        <div class="card-body text-center rank-card">
                                            <h2 class="text-center d-block d-md-none element-title ">
                                                {{ $rank->loser->title }}</h2>
                                            @include('game.partial.my-element-container', [
                                                'rank' => $rank,
                                            ])
                                        </div>
                                    </div>
                                  </div>

                                  @if (config('services.google_ad.enabled') && config('services.google_ad.rank_page') && $index == 4)
                                    <div id="google-ad-1" class="col-12 p-2 d-sm-none">
                                      @include('ads.rank_ad_1', ['id' => 'google-ad-1'])
                                    </div>
                                  @endif

                                @endforeach
                                </div>
                            </b-tab>
                        @endif

                        {{-- Game room rank --}}
                        @if ($gameResult && isset($gameResult->game_room))
                          <b-tab {{ request('tab') == 2 ? 'active' : '' }} @click="clickTab('2')">
                            <template #title>
                              <h5><i class="fa-solid fa-gamepad"></i>&nbsp;多人模式</h5>
                            </template>
                            <div class="container game-room-box bg-secondary text-white" style="max-width: 640px">
                              <h3 class="p-1 my-1 text-center position-relative">排行榜
                                <span v-show="sortByTop" class="btn btn-secondary btn-sm cursor-pointer position-absolute m-0 p-1" @click="changeSortRanks">
                                  <i class="fa-solid fa-arrow-up-wide-short"></i>
                                </span>
                                <span v-show="!sortByTop" class="btn btn-secondary btn-sm cursor-pointer position-absolute m-0 p-1" @click="changeSortRanks">
                                  <i class="fa-solid fa-arrow-down-short-wide"></i>
                                </span>
                              </h3>
                              <div class="p-1 my-1 game-room-container">
                                <div class="d-flex justify-content-between p-2">
                                  <div class="d-flex align-items-center">
                                    <span class="badge badge-pill badge-light mr-1">
                                    #
                                    </span>
                                    暱稱
                                  </div>
                                  <span>積分
                                    <i class="fa-solid fa-circle-question" data-toggle="tooltip" data-placement="top" data-html="true" title="預測正確:連勝*10分<br>預測失敗:-10分"></i>
                                  </span>
                                </div>
                                <hr class="my-1 text-white bg-white">
                                <transition-group name="list-left" tag="div">
                                  <div class="d-flex justify-content-between position-relative align-items-center mx-2" v-if="getSortedRanks" v-for="(rank,index) in getSortedRanks" :key="rank.user_id+':'+rank.rank">
                                    <h6 class="d-flex align-items-center">
                                      <i v-if="rank.rank == 1" class="fa-solid fa-trophy mr-1" style="color:gold"></i>
                                      <i v-else-if="rank.rank == 2" class="fa-solid fa-trophy mr-1" style="color:silver"></i>
                                      <i v-else-if="rank.rank == 3" class="fa-solid fa-trophy mr-1" style="color:chocolate"></i>
                                      <i v-else-if="rank.rank == gameRoomRanks.total_users" class="fa-solid fa-poo mr-1" style="color:gold"></i>
                                      <i v-else-if="rank.rank == gameRoomRanks.total_users -1" class="fa-solid fa-poo mr-1" style="color:silver"></i>
                                      <i v-else-if="rank.rank == gameRoomRanks.total_users -2" class="fa-solid fa-poo mr-1" style="color:chocolate"></i>
                                      <span v-else-if="rank.rank > 0" class="badge badge-pill badge-light mr-1">
                                        @{{rank.rank}}
                                      </span>
                                      <span>
                                        @{{rank.name | cut(10)}}
                                      </span>
                                    </h6>
                                    <h5 class="position-absolute" style="right: 0">
                                      <span data-toggle="tooltip" data-placement="top" :title="$t('game.bet.combo')">
                                        <span v-if="rank.combo >= 10" class="badge badge-pill badge-combo-10">
                                          @{{rank.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                                        </span>
                                        <span v-else-if="rank.combo >= 8" class="badge badge-pill badge-combo-8">
                                          @{{rank.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                                        </span>
                                        <span v-else-if="rank.combo >= 6" class="badge badge-pill badge-combo-6">
                                          @{{rank.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                                        </span>
                                        <span v-else-if="rank.combo >= 4" class="badge badge-pill badge-combo-4">
                                          @{{rank.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                                        </span>
                                        <span v-else-if="rank.combo >= 2" class="badge badge-pill badge-combo-2">
                                          @{{rank.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                                        </span>
                                      </span>
                                      <I-Count-Up :end-val="rank.score" :options="{startVal:rank.score}"></I-Count-Up>
                                    </h5>
                                  </div>
                                </transition-group>
                              </div>
                            </div>
                          </b-tab>
                        @endif

                        {{-- Global Rank --}}
                        <b-tab {{ request('tab') == 1 ? 'active' : '' }} @click="clickTab('1')">
                            <template #title>
                              <h5><i class="fa-solid fa-chart-line"></i>&nbsp;@{{$t('Global Rank')}}</h5>
                            </template>
                            <div class="row">
                                @foreach ($reports as $index => $rank)
                                    @if ($embed || in_array($rank->rank, [1, 2, 3, 4, $reports->total(), $reports->total() - 1]))
                                        <div class="col-12">
                                            <div class="card my-2 card-hover">
                                                <div class="card-header rank-header">
                                                    <span class="text-left w-25 rank-number">{{ $rank->rank }}</span>
                                                    <h2 class="text-center d-none d-md-block w-50 element-title">
                                                        {{ $rank->element->title }}</h2>
                                                    <div class="text-right ml-auto w-auto">
                                                        @if ($rank->win_rate)
                                                            {{ __('rank.win_rate') }}:&nbsp;{{ round($rank->win_rate, 1) }}%
                                                        @else
                                                            {{ __('rank.win_rate') }}:&nbsp;0%
                                                        @endif
                                                    </div>
                                                </div>
                                                <div class="card-body text-center rank-card">
                                                    <div class="row">
                                                        <div class="col-12 col-md-6">
                                                            <h2 class="text-center d-block d-md-none element-title">
                                                                {{ $rank->element->title }}</h2>
                                                            <div
                                                                class="col-12 align-content-center justify-content-center">
                                                                @include('game.partial.global-element-container',['rank' => $rank])
                                                            </div>
                                                        </div>
                                                        <div class="col-12 col-md-6">
                                                            <rank-history-chart
                                                                chart-id="{{ 'global-rank-history-chart-' . $rank->element->id }}"
                                                                element-id="{{ $rank->element->id }}"
                                                                post-serial="{{ $post->serial }}"
                                                                index-rank-endpoint="{{ route('api.rank.index') }}">
                                                            </rank-history-chart>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    @else
                                        <div class="col-12 col-xl-6">
                                            <div class="card my-2 card-hover">
                                                <div class="card-header rank-header">
                                                    <span class="text-left w-25 rank-number">{{ $rank->rank }}</span>
                                                    <h2 class="text-center d-none d-md-block w-50 element-title">
                                                        {{ $rank->element->title }}</h2>
                                                    <div class="text-right ml-auto w-auto">
                                                        @if ($rank->win_rate)
                                                            {{ __('rank.win_rate') }}:&nbsp;{{ round($rank->win_rate, 1) }}%
                                                        @else
                                                            {{ __('rank.win_rate') }}:&nbsp;0%
                                                        @endif
                                                    </div>
                                                </div>
                                                <div class="card-body text-center rank-card">
                                                    <h2 class="text-center d-block d-md-none element-title">
                                                        {{ $rank->element->title }}</h2>
                                                    @if ($rank->rank === 1)
                                                        <div class="col-12 align-content-center justify-content-center">
                                                            @include(
                                                                'game.partial.global-element-container',
                                                                ['rank' => $rank]
                                                            )
                                                        </div>
                                                    @else
                                                        @include('game.partial.global-element-container', [
                                                            'rank' => $rank,
                                                        ])
                                                    @endif
                                                </div>
                                            </div>
                                        </div>
                                    @endif

                                    @if (config('services.google_ad.enabled') && config('services.google_ad.rank_page') && $index == 5)
                                        <div id="google-ad-2" class="col-12 p-4 d-sm-none">
                                          <div class="m-4 p-2">
                                            @include('ads.rank_ad_1', ['id' => 'google-ad-2'])
                                          </div>
                                        </div>
                                    @endif
                                @endforeach
                            </div>

                            @if ($reports && count($reports) == 0)
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
                                    {{ $reports->onEachSide(2)->appends(request()->except('page'))->appends(['tab' => 1])->links() }}
                                </div>
                            </div>
                            <div class="d-block d-md-none">
                                <div class="d-flex justify-content-center pt-2">
                                    @include('layouts.partials.mobile-pagination', [
                                        'paginator' => $reports,
                                    ])
                                </div>
                            </div>
                        </b-tab>


                    </b-tabs>

                    {{-- Comment --}}
                    <hr class="my-4">
                    <h5 id="comments-total" class="d-inline">{{ __('Comment') }}(@{{ meta.total }})</h5>
                    <div class="card mb-4">
                        {{-- Comments --}}
                        <div class="card-body">
                            <div v-for="comment in comments">
                                <div class="d-flex justify-content-between w-100">
                                    {{-- avatar --}}
                                    <div class="avatar-container">
                                        <div class="avatar">
                                            <img v-if="comment.avatar_url" :src="comment.avatar_url"
                                                :alt="comment.nickname">
                                            <img v-else src="{{ asset('storage/default-avatar.webp') }}"
                                                :alt="comment.nickname">
                                        </div>
                                    </div>
                                    <div class="comment-container">
                                        {{-- nickname --}}
                                        <div class="d-flex justify-content-between">
                                            <span class="text-black-50 font-size-large" style="overflow-wrap:anywhere">
                                                <h5>@{{ comment.nickname }}</h5>
                                            </span>
                                            <div class="ml-auto">
                                                <div class="text-align-end">
                                                    <div class="d-flex">
                                                        {{-- timestamp --}}
                                                        <div class="text-right text-muted mr-4">
                                                            @{{ comment.created_at | formNow }}
                                                        </div>
                                                        {{-- options --}}
                                                        <div class="dropdown">
                                                            <span href="#" role="button" id="reportDropdown"
                                                                data-toggle="dropdown" aria-haspopup="true"
                                                                aria-expanded="false">
                                                                <i class="fa-xl fa-solid fa-ellipsis-vertical cursor-pointer text-center"
                                                                    style="width: 20px"></i>
                                                            </span>

                                                            <div class="dropdown-menu dropdown-menu-right"
                                                                aria-labelledby="reportDropdown">
                                                                <a class="dropdown-item"
                                                                    @click.prevent="reportComment(comment)"
                                                                    href="#"><i
                                                                        class="fa-solid fa-triangle-exclamation"></i>&nbsp;{{ __('comment.report') }}</a>
                                                            </div>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                        {{-- champions --}}
                                        <h5 class="overflow-hidden" v-if="comment.champions.length > 0">
                                            <template v-for="champion in comment.champions">
                                                <h5 class="badge badge-secondary mr-1 mb-1 rounded-0"
                                                    :title="champion">@{{ champion }}</h5>
                                            </template>
                                        </h5>
                                        {{-- comment --}}
                                        <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar"
                                            style="max-height: 200px;">@{{ comment.content }}</p>
                                    </div>
                                </div>

                                {{-- display for mobile (width < 576px) --}}
                                <div class="d-block d-sm-none">
                                    {{-- avatar --}}
                                    <div class="d-flex">
                                        <div class="" style="margin-right:20px">
                                            <div class="avatar">
                                                <img v-if="comment.avatar_url" :src="comment.avatar_url"
                                                    :alt="comment.nickname">
                                                <img v-else src="{{ asset('storage/default-avatar.webp') }}"
                                                    :alt="comment.nickname">
                                            </div>
                                        </div>

                                        <div>
                                            {{-- nickname --}}
                                            <span class="text-black-50 font-size-large text-break">
                                                <h5> @{{ comment.nickname }}</h5>
                                            </span>
                                            {{-- timestamp --}}
                                            <div class="text-muted">
                                                @{{ comment.created_at | formNow }}
                                            </div>
                                        </div>
                                        <div class="ml-auto">
                                            <div class="text-align-end">
                                                <div class="dropdown">
                                                    <span href="#" role="button" id="reportDropdown"
                                                        data-toggle="dropdown" aria-haspopup="true"
                                                        aria-expanded="false">
                                                        <i class="fa-xl fa-solid fa-ellipsis-vertical cursor-pointer text-center"
                                                            style="width: 20px"></i>
                                                    </span>

                                                    <div class="dropdown-menu dropdown-menu-right"
                                                        aria-labelledby="reportDropdown">
                                                        <a class="dropdown-item" href="#"
                                                            @click.prevent="reportComment(comment)"><i
                                                                class="fa-solid fa-triangle-exclamation"></i>&nbsp;{{ __('comment.report') }}</a>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    {{-- champions --}}
                                    <h5 class="overflow-hidden" v-if="comment.champions.length">
                                        <template v-for="champion in comment.champions">
                                            <h5 class="badge badge-secondary mr-1 mb-1 rounded-0" :title="champion">
                                                @{{ champion }}</h5>
                                        </template>
                                    </h5>

                                    {{-- comment --}}
                                    <p class="break-all white-space-pre-line overflow-scroll hide-scrollbar"
                                        style="max-height: 200px;">@{{ comment.content }}</p>
                                </div>
                                <hr class="my-1">
                            </div>
                            {{-- Pagination --}}
                            <b-pagination v-if="meta.last_page > 1" v-model="meta.current_page" :total-rows="meta.total"
                                :per-page="meta.per_page" @input="changePage"
                                class="justify-content-center"></b-pagination>
                        </div>

                        {{-- leave a comment --}}
                        <div class="card-body">
                            <div class="row">
                                {{-- comment input --}}
                                <div class="col-12">
                                    <div class="">
                                        {{ __('Nickname') }}: <span class="text-black-50 font-size-large text-break">
                                            <h5 class="d-inline" v-if="!anonymous">@{{ profile.nickname }}</h5>
                                            <h5 class="d-inline" v-if="anonymous">****</h5>
                                        </span>
                                        {{-- anonymous mode --}}
                                        <div class="mr-auto from-check" v-if="profile.is_auth">
                                            <input class="from-check" type="checkbox" id="anonymous"
                                                v-model="anonymous">
                                            <label class="from-control"
                                                for="anonymous">{{ __('comment.anonymous') }}</label>
                                        </div>
                                    </div>
                                    <div class="overflow-hidden" v-if="profile.champions.length > 0">
                                        {{ __('Vote result') }}:
                                        {{-- champions --}}
                                        <h5 class="d-inline">
                                            <template v-for="champion in profile.champions">
                                                <span class="badge badge-secondary mr-1 mb-1"
                                                    :title="champion">@{{ champion }}</span>
                                            </template>
                                        </h5>
                                    </div>
                                </div>
                                {{-- words --}}
                                <div class="col-12">
                                    <div class="form-group">
                                        <textarea class="form-control comment-input-field" rows="3"
                                            maxlength="{{ config('setting.comment_max_length') }}" placeholder="{{ __('comment.leave_comment') }}"
                                            style="resize: none" v-model="commentInput"></textarea>
                                        <span> (@{{ commentWords }}/@{{ commentMaxLength }})</span>
                                    </div>
                                </div>
                                {{-- submit --}}
                                <div class="col-12">
                                    <button type="button" class="btn btn-primary"
                                        :disabled="!validComment || isSubmiting"
                                        @click="submitComment">{{ __('comment.submit') }}</button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {{-- right part: ads --}}
                <div class="d-none d-lg-block col-lg-2">
                  @if(config('services.google_ad.enabled') && config('services.google_ad.rank_page'))
                    <div class="p-lg-2 p-xl-3">
                        @include('ads.rank_ad_sides')
                    </div>
                  @endif
                </div>
            </div>
        </div>
    </rank>
@endsection


@section('footer')
  <script async
    src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
    crossorigin="anonymous"></script>
@endsection
