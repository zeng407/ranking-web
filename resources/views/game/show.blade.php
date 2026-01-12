@extends('layouts.app', [
    'title' => $post->isPublic() ? $post : __('title.access'),
    'ogTitle' => $post->isPublic() ? $post->title : null,
    'ogImage' => $post->isPublic() ? $element?->getLowThumbUrl() : null,
    'ogDescription' => $post->isPublic() ? $post->description : null,
    ])


@section('header')
  @if($post->isPublic())
    <script type="application/ld+json">
    {
        "@context": "https://schema.org",
        "@type": "NewsArticle",
        "headline": "{{get_page_title($post->title)}}",
        "datePublished": "{{$post->created_at->toIso8601String()}}",
        "dateModified": "{{$post->updated_at->toIso8601String()}}"
    }
    </script>
  @endif
  <script src="https://embed.twitch.tv/embed/v1.js"></script>
  @if(config('services.google_ad.enabled') && config('services.google_ad.game_page') && !is_skip_ad())
    {{-- ads --}}
    <script data-cfasync="false" async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js" crossorigin="anonymous"></script>
    <script data-cfasync="false">
      // window.googletag = window.googletag || {cmd: []};
      // googletag.cmd.push(function() {
      //   var slot = googletag.defineSlot('/23307026516/game_ad_top', [[300, 100], [320, 100], [120, 90], [220, 90], [300, 75], 'fluid'], 'div-gpt-ad-1750913246554-0').addService(googletag.pubads());
      //   googletag.pubads().setCentering(true);
      //   googletag.pubads().enableSingleRequest();
      //   googletag.enableServices();
      //   setInterval(() => {
      //     googletag.display("div-gpt-ad-1750913246554-0");
      //     googletag.pubads().refresh([slot]);
      //   }, 30 * 1000); // 30 seconds
      // });

      // googletag.cmd.push(function() {
      //   var slot = googletag.defineSlot('/23307026516/game_ad_top/game_ad_top_2', [[320, 100], [220, 90], [300, 75], [300, 100], [120, 90], [468, 60]], 'div-gpt-ad-1751199509158-0').addService(googletag.pubads());
      //   googletag.pubads().enableSingleRequest();
      //   googletag.enableServices();

      //   setInterval(() => {
      //     googletag.display("div-gpt-ad-1751199509158-0");
      //     googletag.pubads().refresh([slot]);
      //   }, 30 * 1000); // 30 seconds
      // });
    </script>
  @endif
@endsection

@section('content')
    <game
        inline-template
        post-serial="{{$serial}}"
        user-last-game-serial="{{$userLastGameSerial}}"
        :require-password="{{json_encode($requiredPassword)}}"
        get-rank-route="{{route('game.rank', '_serial')}}"
        get-game-setting-endpoint="{{route('api.game.setting', $serial)}}"
        next-round-endpoint="{{route('api.game.next-round', '_serial')}}"
        create-game-endpoint="{{route('api.game.create')}}"
        vote-game-endpoint="{{route('api.game.vote')}}"
        access-endpoint="{{route('api.game.access', $serial)}}"
        props-game-room-serial="{{$gameRoom?->serial}}"
        get-room-endpoint="{{route('api.game-room.get', '_serial')}}"
        get-room-votes-endpoint="{{route('api.game-room.get-votes', '_serial')}}"
        get-room-user-endpoint="{{route('api.game-room.get-user', '_serial')}}"
        update-room-profile-endpoint="{{route('api.game-room.update-profile', '_serial')}}"
        bet-endpoint="{{route('api.game-room.bet', '_serial')}}"
        get-game-elements-endpoint="{{route('api.game.elements', '_serial')}}"
        batch-vote-endpoint="{{route('api.game.batch-vote')}}"
        :props-enable-client-mode="true"
    >
    <div class="container-fluid hide-scrollbar game-dark-theme pt-2" v-cloak>
        @if(!$post->is_censored && config('services.google_ad.enabled') && config('services.google_ad.game_page') && !is_skip_ad())
        {{-- ads --}}

        {{-- <div id="google-ad-container" class="row overflow-hidden position-relative"> --}}
          <!-- /23307026516/game_ad_top -->
          {{-- <div v-if="isMobileScreen" class="col-12 col-sm-6 text-center my-2" id='div-gpt-ad-1750913246554-0' style='min-width: 120px; height: 100px;'>
            <script>
              googletag.cmd.push(function() { googletag.display('div-gpt-ad-1750913246554-0'); });
            </script>
          </div> --}}

          <!-- /23307026516/game_ad_top/game_ad_top_2 -->
          {{-- <div v-if="isMobileScreen" class="col-12 col-sm-6 text-center my-2" id='div-gpt-ad-1751199509158-0' style='min-width: 120px; height: 100px;'>
            <script>
              googletag.cmd.push(function() { googletag.display('div-gpt-ad-1751199509158-0'); });
            </script>
          </div> --}}
        {{-- </div> --}}

        <div v-if="isMobileScreen" id="google-ad-container" style="height: 100px; z-index:1" class="overflow-hidden position-relative">
          <div v-if="!refreshAD && game" id="google-ad" class="my-2 text-center">
              @include('ads.game_ad_mobile')
          </div>
        </div>
        @endif

        {{-- creating game loading --}}
        <div v-show="creatingGame">
          <div class="d-flex justify-content-center align-items-center flex-column" style="height: 100vh">
            <img src="{{ asset('storage/logo.png') }}" class="updown-animation" alt="logo" style="width: 50px; height: 50px;">
            <div>
              @{{ $t('game.creating') }}
              <div class="spinner-border text-primary" role="status">
                <span class="sr-only">Loading...</span>
              </div>
            </div>
          </div>
        </div>

        {{-- finishing game loading --}}
        <div v-show="finishingGame && !isBetGameClient"
             class="finishing-game-overlay"
             style="position: fixed; top: 0; left: 0; width: 100vw; height: 100vh; z-index: 9999;">

          <div class="d-flex justify-content-center align-items-center flex-column w-100 h-100">
            <img src="{{ asset('storage/logo.png') }}" class="updown-animation" alt="logo" style="width: 50px; height: 50px;">
            <div v-if="gameResultUrl === ''" class="finishing-text">
              @{{ $t('game.finishing') }}
              {{-- spinner --}}
              <div class="spinner-border text-primary" role="status">
                <span class="sr-only">Loading...</span>
              </div>
            </div>
            <div v-else class="finishing-text">
              <a :href="gameResultUrl">@{{ $t('game.checkout_result') }}</a>
            </div>
          </div>
        </div>

        {{-- main --}}
        <div class="row">
          {{-- left part: ads --}}
          <div class="col-xl-2 d-none d-xl-block">
            @if(config('services.google_ad.enabled') && config('services.google_ad.game_page'))
            <div v-if="!refreshAD && game && !isMobileScreen" class="p-lg-1 p-xl-2">
                @include('ads.game_ad_sides')
            </div>
            @endif
          </div>

          {{-- elements --}}
          <div class="col-xl-8 col-12"
            style="min-height: 800px">

            {{-- bet success animation: firework --}}
            <div class="pyro" v-if="showFirework">
              <div class="before" style="z-index: 110"></div>
              <div class="after" style="z-index: 110" ></div>
            </div>
            <transition-group name="bet-result-animation" tag="div" class="position-absolute top-50 left-50 translate-50-50 w-100 bg-dark-50 overflow-hidden" style="z-index: 110">
              <div v-if="showFirework" key="bet-success">
                <h1 class="text-white text-center">@{{$t('game_room.predict_success')}}</h1>
              </div>
            </transition-group>
            <transition-group name="bet-result-animation" tag="div" class="position-absolute top-50 left-50 translate-50-50 w-100 bg-dark-50 overflow-hidden" style="z-index: 110">
              <div v-if="showBetFailed" key="bet-success">
                <h1 class="text-white text-center">@{{$t('game_room.predict_fail')}}</h1>
              </div>
            </transition-group>

            <div v-if="isGameRoomFinished">
              <div class="d-flex justify-content-center align-content-center mt-4">
                <h3>@{{$t('game_room.vote_ends')}}</h3>
              </div>
              <div class="d-flex justify-content-center align-content-center mt-4">
                <a :href="getRankResultUrl()" >查看排行</a>

              </div>
            </div>

            {{-- game heading --}}
            <div v-if="game && !finishingGame">
              <div id="game-title" class="text-center text-break mt-1 game-title">{{$post->title}}</div>
              <div class="d-none d-sm-flex" style="flex-flow: row wrap">
                <h3 class="text-nowrap" style="width: 20%">
                  <span v-if="isBetGameClient && !isVoting">@{{$t('game_room.guess_winner')}}</span>
                  <span v-if="isBetGameClient && isVoting">@{{$t('game_room.waiting_result')}}</span>
                </h3>

                <h3 class="text-center align-self-center" style="width: 60%">
                  <span v-if="roundTitleCount <= 2">@{{ $t('game_round_final') }}</span>
                  <span v-else-if="roundTitleCount <= 4">@{{ $t('game_round_semifinal') }}</span>
                  <span v-else-if="roundTitleCount <= 8">@{{ $t('game_round_quarterfinal') }}</span>
                  <span v-else-if="roundTitleCount <= 1024">@{{ $t('game_round_of', {round: roundTitleCount}) }}</span>
                  @{{ displayCurrentRound }}&nbsp;/&nbsp;@{{ displayTotalRound }}
                </h3>

                <h3 class="d-flex justify-content-end text-right align-self-center" style="width: 20%">
                  <div class="badge badge-pill badge-light px-3 py-1 shadow-sm d-flex align-items-center mr-2"
                    style="font-size: 0.85rem; border: 1px solid #dee2e6; max-width: 200px"
                    title="Timer">
                    <div class="d-flex align-items-center text-secondary mr-2" style="opacity: 0.6;">
                      <i class="fa-regular fa-clock" style="width: 20px; text-align: center;"></i>
                    </div>
                    <div class="d-flex align-items-center">
                      <span class="text-muted small" style="font-size: 0.95rem;">
                        @{{ displayTimer }}
                      </span>
                    </div>
                  </div>

                  <div class="badge badge-pill badge-light px-3 py-1 shadow-sm d-flex align-items-center"
                    style="font-size: 0.85rem; border: 1px solid #dee2e6; max-width: 200px"
                    :title="$t('game.remaining.hint', {total: displayTotalElements, remaining: displayRemainElements})">

                    <div class="d-flex align-items-center text-secondary mr-1" style="opacity: 0.6;">
                      <i class="fas fa-chess-pawn mr-1" style="width: 20px; text-align: center;"></i>
                      <span class="font-weight-bold">@{{ displayTotalElements }}</span>
                    </div>

                    <div class="mx-2 bg-secondary" style="width: 1px; height: 12px; opacity: 0.2;"></div>

                    <div class="d-flex align-items-center">
                      <span class="text-muted small mr-1 d-none d-sm-inline">@{{ $t('game.remaining') }}</span>
                      <span class="text-muted small" style="font-size: 0.95rem;">
                          @{{ displayRemainElements }}
                      </span>
                    </div>
                  </div>
                </h3>
              </div>
            </div>

            {{-- playground --}}
            <div class="row overflow-hidden" v-if="game && !finishingGame && (!gameRoom || !gameRoom.is_game_completed)"
              :style="{'height': (isFixedGameHeight ? (gameBodyHeight + 'px') : 'auto')}">
              <!--left part-->
              <div class="col-12 col-sm-6 pr-sm-1 mb-2 mb-sm-0" id="left-part">
                <transition name="slide-in-left">
                  <div v-if="animationShowLeftPlayer" class="card game-player left-player vs-card" id="left-player">
                    <div v-show="isImageSource(le)" class="game-image-container" v-cloak>
                      <flex-image
                        :key="le.id"
                        v-show="!leftImageLoaded"
                        :element-id="le.id"
                        :thumb-url="getLowThumbUrl(le)"
                        :image-key="le.id"
                        :imgur-url="le.imgur_url"
                        :alt="le.title"
                        :height="elementHeight"
                      ></flex-image>
                      <viewer ref="leftViewer" :options="viewerOptions">
                        <flex-image
                          :key="le.id"
                          v-show="leftImageLoaded"
                          :handle-loaded="handleLeftLoaded"
                          :element-id="le.id"
                          :thumb-url="getThumbUrl(le)"
                          :image-key="le.id"
                          :imgur-url="le.imgur_url"
                          :alt="le.title"
                          :height="elementHeight"
                        ></flex-image>
                      </viewer>
                    </div>
                    <div v-if="isYoutubeSource(le) && !isDataLoading" class="d-flex hover-effect" @mouseover="videoHoverIn(le, re, true)" @mouseleave="videoHoverOut(le, re, true)">
                      <youtube :video-id="le.video_id" width="100%" :height="elementHeight" :ref="le.id"
                        :player-vars="{ controls: 1, autoplay: !isMobileScreen && !isBetGameClient, rel: 0 , origin: origin, playlist: le.video_id, start:le.video_start_second, end:le.video_end_second }">
                      </youtube>
                    </div>
                    <div v-else-if="isYoutubeEmbedSource(le) && !isDataLoading" class="d-flex">
                      <youtube-embed v-if="le" :element="le" width="100%" :height="elementHeight" />
                    </div>
                    <div v-else-if="isBilibiliSource(le) && !isDataLoading" class="d-flex">
                      <bilibili-video v-if="le" :element="le" width="100%" :autoplay="false" :muted="false" :height="elementHeight"/>
                    </div>
                    <div v-else-if="isTwitchVideoSource(le) && !isDataLoading" class="d-flex">
                      <div :id="'twitch-video-'+le.id" class="w-100 twitch-container"></div>
                    </div>
                    <div v-else-if="isTwitchClipSource(le) && !isDataLoading" class="d-flex twitch-container">
                      <iframe :src="'https://clips.twitch.tv/embed?clip='+le.video_id+'&parent='+host+'&autoplay=false'"
                            :height="elementHeight"
                            width="100%"
                            allowfullscreen></iframe>
                    </div>
                    <div v-else-if="isVideoSource(le) && !isDataLoading" class="d-flex">
                      <video id="left-video-player" v-if="isMobileScreen" width="100%" :height="elementHeight" loop controls playsinline :src="le.source_url" :poster="le.thumb_url"></video>
                      <video id="left-video-player" v-else width="100%" :height="elementHeight" loop controls playsinline muted :src="le.source_url" :poster="le.thumb_url"></video>
                    </div>
                    <div class="card-body text-center">
                      <div class="my-1 overflow-scroll hide-scrollbar" style="max-height: 35px" v-if="isMobileScreen">
                        <h1 class="my-1 font-size-small">@{{ le.title }}</h1>
                      </div>
                      <div class="my-1 overflow-scroll hide-scrollbar" style="height: 120px" v-else>
                        <h1 class="my-1 game-element-title">@{{ le.title }}</h1>
                      </div>
                      <button id="left-btn" class="btn btn-vote-left vote-button btn-block d-none d-sm-block" :disabled="isVoting"
                        @click="leftWin">
                        <i class="fa-solid fa-2x fa-thumbs-up"></i>
                        <h3 class="d-inline-block m-0" v-if="showGameRoomVotes">
                          <I-Count-Up :end-val="leftVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                          (<I-Count-Up :end-val="leftVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                        </h3>
                      </button>
                      <div class="row" v-if="isYoutubeSource(le) || isVideoUrlSource(le)">
                        {{-- mobile --}}
                        <div class="col-3">
                          <button class="btn btn-outline-primary btn-lg btn-block d-block d-sm-none btn-hover-bounce" :class="{active: isLeftPlaying}" :disabled="isVoting"
                            @click="leftPlay()">
                            <i class="fas fa-volume-mute" v-show="!isLeftPlaying"></i>
                            <i class="fas fa-volume-up" v-show="isLeftPlaying"></i>
                          </button>
                        </div>
                        <div class="col-9">
                          <button class="btn btn-vote-left btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                            @click="leftWin"><i class="fa-solid fa-thumbs-up"></i>
                            <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                              <I-Count-Up :end-val="leftVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                              (<I-Count-Up :end-val="leftVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                            </p>
                          </button>
                        </div>
                      </div>

                      <div v-else>
                        <button class="btn btn-vote-left btn-block btn-lg d-block d-sm-none" :disabled="isVoting"
                          @click="leftWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                          <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                            <I-Count-Up :end-val="leftVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                            (<I-Count-Up :end-val="leftVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                          </p>
                        </button>
                      </div>
                    </div>
                  </div>
                </transition>
              </div>

              <!-- mobile rounds heading -->
              <transition name="slide-in-left">
                <div v-if="animationShowRoundSession" id="rounds-session" class="col-12 d-sm-none">
                  <div class="d-flex d-sm-none justify-content-between" style="flex-flow: row wrap">
                    <h5 class="m-0">
                      <span v-if="roundTitleCount <= 2">@{{ $t('game_round_final') }}</span>
                      <span v-else-if="roundTitleCount <= 4">@{{ $t('game_round_semifinal') }}</span>
                      <span v-else-if="roundTitleCount <= 8">@{{ $t('game_round_quarterfinal') }}</span>
                      <span v-else-if="roundTitleCount <= 1024">@{{ $t('game_round_of', {round: roundTitleCount}) }}</span>
                        @{{ displayCurrentRound }}&nbsp;/&nbsp;@{{ displayTotalRound }}
                    </h5>
                    <h5 v-if="isBetGameClient">
                      <span v-if="isBetGameClient && !isVoting">@{{$t('game_room.guess_winner')}}</span>
                      <span v-if="isBetGameClient && isVoting">@{{$t('game_room.waiting_result')}}</span>
                    </h5>

                    <h5 class="">
                      <div class="badge badge-pill badge-light px-3 py-1 shadow-sm d-flex align-items-center"
                        style="font-size: 0.85rem; border: 1px solid #dee2e6; max-width: 200px"
                        :title="$t('game.remaining.hint', {total: displayTotalElements, remaining: displayRemainElements})">

                        <div class="d-flex align-items-center text-secondary mr-1" style="opacity: 0.6;">
                          <i class="fas fa-chess-pawn mr-1" style="width: 20px; text-align: center;"></i>
                          <span class="font-weight-bold">@{{ displayTotalElements }}</span>
                        </div>

                        <div class="mx-2 bg-secondary" style="width: 1px; height: 12px; opacity: 0.2;"></div>

                        <div class="d-flex align-items-center">
                          <span class="text-muted small mr-1 d-none d-sm-inline">@{{ $t('game.remaining') }}</span>
                          <span class="text-muted small" style="font-size: 0.95rem;">
                              @{{ displayRemainElements }}
                          </span>
                        </div>
                      </div>
                    </h5>

                  </div>
                </div>
              </transition>

              <!--right part-->
              <div class="col-12 col-sm-6 pl-sm-1 mb-4 mb-sm-0" id="right-part">
                <transition :name="isMobileScreen ? 'slide-in-left' : 'slide-in-right'">
                  <div v-if="animationShowRightPlayer" class="card game-player right-player vs-card" id="right-player" :class="{ 'flex-column-reverse': isMobileScreen, 'mb-4': isMobileScreen}">
                    <div v-show="isImageSource(re)" class="game-image-container" v-cloak>
                      <flex-image
                        v-show="!rightImageLoaded"
                        :key="re.id"
                        :element-id="re.id"
                        :thumb-url="getLowThumbUrl(re)"
                        :image-key="re.id"
                        :imgur-url="re.imgur_url"
                        :alt="re.title"
                        :height="elementHeight"
                      ></flex-image>
                      <viewer ref="rightViewer" :options="viewerOptions">
                        <flex-image
                          v-show="rightImageLoaded"
                          :key="re.id"
                          :handle-loaded="handleRightLoaded"
                          :element-id="re.id"
                          :thumb-url="getThumbUrl(re)"
                          :image-key="re.id"
                          :imgur-url="re.imgur_url"
                          :alt="re.title"
                          :height="elementHeight"
                        ></flex-image>
                      </viewer>
                    </div>
                    <div v-if="isYoutubeSource(re) && !isDataLoading" class="d-flex hover-effect" @mouseover="videoHoverIn(re, le, false)" @mouseleave="videoHoverOut(re, le, false)">
                      <youtube :video-id="re.video_id" width="100%" :height="elementHeight" :ref="re.id"
                        :player-vars="{ controls: 1, autoplay: !isMobileScreen && !isBetGameClient, rel: 0, origin: origin,  playlist: re.video_id, start:re.video_start_second, end:re.video_end_second}">
                      </youtube>
                    </div>
                    <div v-else-if="isYoutubeEmbedSource(re) && !isDataLoading" class="d-flex">
                      <youtube-embed v-if="re" :element="re" width="100%" :height="elementHeight"/>
                    </div>
                    <div v-else-if="isTwitchVideoSource(re) && !isDataLoading" class="d-flex">
                      <div :id="'twitch-video-'+re.id" class="w-100 twitch-container"></div>
                    </div>
                    <div v-else-if="isTwitchClipSource(re) && !isDataLoading" class="d-flex twitch-container">
                      <iframe :src="'https://clips.twitch.tv/embed?clip='+re.video_id+'&parent='+host+'&autoplay=false'"
                            :height="elementHeight"
                            width="100%"
                            allowfullscreen></iframe>
                    </div>
                    <div v-else-if="isBilibiliSource(re) && !isDataLoading" class="d-flex">
                      <bilibili-video v-if="re" :element="re" width="100%" :autoplay="false" :muted="false" :height="elementHeight"/>
                    </div>
                    <div v-else-if="isVideoSource(re) && !isDataLoading" class="d-flex">
                      <video id="right-video-player" v-if="isMobileScreen" width="100%" :height="elementHeight" loop controls playsinline :src="re.source_url" :poster="re.thumb_url"></video>
                      <video id="right-video-player" v-else width="100%" :height="elementHeight" loop controls playsinline muted :src="re.source_url" :poster="re.thumb_url"></video>
                    </div>

                    <!-- reverse when device size width less md(768px)-->
                    <div class="card-body text-center"
                      :class="{ 'flex-column-reverse': isMobileScreen, 'd-flex': isMobileScreen }">
                      <div class="my-1 flex-column-reverse d-flex overflow-scroll hide-scrollbar" style="max-height: 35px" v-if="isMobileScreen">
                        <h1 class="my-1 font-size-small">@{{ re.title }}</h1>
                      </div>
                      <div class="my-1 overflow-scroll hide-scrollbar" style="height: 120px" v-else>
                        <h1 class="my-1 game-element-title">@{{ re.title }}</h1>
                      </div>
                      <button id="right-btn" class="btn btn-vote-right vote-button btn-block d-none d-sm-block" :disabled="isVoting"
                        @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                        <h3 class="d-inline-block m-0" v-if="showGameRoomVotes">
                          <I-Count-Up :end-val="rightVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                          (<I-Count-Up :end-val="rightVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                        </h3>
                      </button>
                      <div class="row" v-if="isYoutubeSource(re) || isVideoUrlSource(re)">
                        {{-- mobile --}}
                        <div class="col-3">
                          <button class="btn btn-outline-danger btn-lg btn-block d-block d-sm-none btn-hover-bounce" :class="{active: isRightPlaying}" :disabled="isVoting"
                            @click="rightPlay()">
                            <i class="fas fa-volume-mute" v-show="!isRightPlaying"></i>
                            <i class="fas fa-volume-up" v-show="isRightPlaying"></i>
                          </button>
                        </div>
                        <div class="col-9">
                          <button class="btn btn-vote-right btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                            @click="rightWin"><i class="fa-solid fa-thumbs-up"></i>
                            <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                              <I-Count-Up :end-val="rightVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                              (<I-Count-Up :end-val="rightVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                            </p>
                          </button>
                        </div>
                      </div>
                      <div v-else>
                        <button class="btn btn-vote-right btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                          @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                          <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                            <I-Count-Up :end-val="rightVotes" :options="{'duration':0.5}" :delay="0"></I-Count-Up>
                            (<I-Count-Up :end-val="rightVotesPercentage" :options="{'duration':0.5}" :delay="0"></I-Count-Up>%)
                          </p>
                        </button>
                      </div>
                    </div>
                  </div>
                </transition>
              </div>

            </div>
          </div>


          {{-- right part: game room --}}
          <div v-if="gameRoom && !runInBackGameRoom" id="game-room"
            class="col-12 col-xl-2 bg-secondary text-white mt-2 mt-xl-0 game-room-box position-relative">
            <div class="game-room-container">
              {{-- game room --}}
              <div class="p-1 my-1" v-if="isBetGameClient">
                <input v-show="isEditingNickname" id="newNickname" autocomplete="off" v-model="newNickname" type="text" class="form-control badge-secondary bg-secondary-onfocus font-size-16" maxlength="10">
                <div class="text-center" v-show="isEditingNickname">(@{{ $t('game_room.nickname_hint')}})</div>
                <h5 class="d-flex justify-content-center">
                  <div class="position-relative">
                    <u class="cursor-pointer" v-if="!isEditingNickname" @click="toggleEditNickname">@{{gameRoom.user.name | cut(10)}}</u>
                    {{-- edit name --}}
                    <button v-if="!isEditingNickname && !gameRoom.is_game_completed" class="btn btn-secondary btn-sm position-absolute m-0 p-1 px-1" @click="toggleEditNickname">
                      <i class="fa-solid fa-pen-to-square"></i>
                    </button>
                    <div v-if="isEditingNickname && !gameRoom.is_game_completed">
                      <button class="btn btn-secondary m-0 p-0 px-1" :disabled="!newNickname" @click="saveNickname">
                        <i class="fa-solid fa-check"></i>
                      </button>
                      <button class="btn btn-secondary m-0 p-0 px-1" @click="toggleEditNickname">
                        <i class="fa-solid fa-xmark"></i>
                      </button>
                    </div>
                  </div>
                </h5>
                <div class="d-flex justify-content-between">
                  <span>@{{$t('game_room.point')}}:</span>
                  <div>
                    <span data-toggle="tooltip" data-placement="top" :title="$t('game.bet.combo')">
                      <span v-if="gameRoom.user.combo >= 10" class="badge badge-pill badge-combo-10">
                        @{{gameRoom.user.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                      </span>
                      <span v-else-if="gameRoom.user.combo >= 8" class="badge badge-pill badge-combo-8">
                        @{{gameRoom.user.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                      </span>
                      <span v-else-if="gameRoom.user.combo >= 6" class="badge badge-pill badge-combo-6">
                        @{{gameRoom.user.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                      </span>
                      <span v-else-if="gameRoom.user.combo >= 4" class="badge badge-pill badge-combo-4">
                        @{{gameRoom.user.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                      </span>
                      <span v-else-if="gameRoom.user.combo >= 2" class="badge badge-pill badge-combo-2">
                        @{{gameRoom.user.combo}}&nbsp;<i class="fa-solid fa-fire"></i>
                      </span>
                    </span>
                    <I-Count-Up :end-val="gameRoom.user.score"></I-Count-Up>
                  </div>
                </div>
                <div class="d-flex justify-content-between">
                  <span>@{{$t('game_room.rank_number')}} / @{{$t('game_room.rank_players')}}:</span>
                  <span v-if="gameRoom.user.rank > 0">
                    <I-Count-Up :end-val="gameRoom.user.rank"></I-Count-Up>
                    <span>/</span>
                    <I-Count-Up :end-val="gameBetRanks.total_users"></I-Count-Up>
                  </span>
                  <span v-if="gameRoom.user.rank == 0">無 / @{{gameBetRanks.total_users}}</span>
                </div>
                <div class="d-flex justify-content-between">
                  <span>@{{$t('game_room.win_rate')}}:</span>
                  <span>
                    <I-Count-Up :end-val="Number(gameRoom.user.accuracy)" :options="{suffix:'%', decimalPlaces:'2'}"></I-Count-Up>
                  </span>
                </div>
                <div class="d-flex justify-content-between">
                  <span>@{{$t('game_room.predict_success')}} / @{{$t('game_room.predict_times')}}</span>
                  <span>
                    <I-Count-Up :end-val="gameRoom.user.total_correct"></I-Count-Up>
                    /
                    <I-Count-Up :end-val="gameRoom.user.total_played"></I-Count-Up>
                  </span>
                </div>
              </div>
              {{-- room rank --}}
              <div class="p-1 my-1">
                <h5 class="text-center position-relative">
                  @{{ $t('game_room.leaderboard') }}
                  <span v-show="sortByTop" class="btn btn-secondary btn-sm cursor-pointer position-absolute m-0 p-1" @click="changeSortRanks">
                    <i class="fa-solid fa-arrow-up-wide-short"></i>
                  </span>
                  <span v-show="!sortByTop" class="btn btn-secondary btn-sm cursor-pointer position-absolute m-0 p-1" @click="changeSortRanks">
                    <i class="fa-solid fa-arrow-down-short-wide"></i>
                  </span>
                  <span v-if="isBetGameHost" class="btn btn-sm btn-secondary cursor-pointer position-absolute p-0"
                    data-toggle="tooltip" data-placement="left" :title="$t('game_room.minimize_game')"
                    id="minimize-game-room"
                    style="right:20px"
                    @click="minimizeGameRoom">
                    <i class="fa-solid fa-minus"></i>
                  </span>
                  <span v-if="isBetGameHost" class="btn btn-sm btn-secondary cursor-pointer position-absolute p-0"
                    data-toggle="tooltip" data-placement="left" :title="$t('game_room.close_game')"
                    id="close-game-room"
                    style="right:0"
                    @click="closeGameRoom">
                    <i class="fa-solid fa-xmark"></i>
                  </span>
                </h5>
                <h5 class="d-flex justify-content-between bet-rank-broad">
                  <div class="d-flex align-items-center">
                    <span class="badge badge-pill badge-light mr-1">
                    #
                    </span>
                    @{{$t('game_room.nickname')}}
                  </div>
                  <span>@{{$t('game_room.point')}}
                    <i class="fa-solid fa-circle-question" data-toggle="tooltip" data-placement="left" data-html="true" :title="$t('game_room.point_description')"></i>
                  </span>
                </h5>
                <hr class="my-1 text-white bg-white">
                <transition-group name="list-left" tag="div">
                  <div class="d-flex justify-content-between position-relative align-items-center" v-if="getSortedRanks" v-for="(rank,index) in getSortedRanks" :key="rank.user_id+':'+rank.rank">
                    <h5 class="d-flex align-items-center bet-rank-broad">

                      <i v-if="rank.rank == 1" class="fa-solid fa-trophy mr-1" style="color:gold"></i>
                      <i v-else-if="rank.rank == 2" class="fa-solid fa-trophy mr-1" style="color:silver"></i>
                      <i v-else-if="rank.rank == 3" class="fa-solid fa-trophy mr-1" style="color:chocolate"></i>
                      <i v-else-if="rank.rank > 0 && rank.rank == gameBetRanks.total_users" class="fa-solid fa-poo mr-1" style="color:gold"></i>
                      <i v-else-if="rank.rank > 0 && rank.rank == gameBetRanks.total_users -1" class="fa-solid fa-poo mr-1" style="color:silver"></i>
                      <i v-else-if="rank.rank > 0 && rank.rank == gameBetRanks.total_users -2" class="fa-solid fa-poo mr-1" style="color:chocolate"></i>
                      <span v-else-if="rank.rank > 0" class="badge badge-pill badge-light mr-1">
                        @{{rank.rank}}
                      </span>
                      <span :class="{'text-warning':isSameUser(rank)}">
                        @{{rank.name | cut(10)}}
                      </span>
                    </h5>
                    <h5 class="position-absolute bet-rank-broad" style="right: 0">
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
                      <span v-b-tooltip.hover.html.left="tipMethod(rank)">
                        <I-Count-Up :end-val="rank.score" :options="{startVal:rank.score}"></I-Count-Up>
                      </span>
                    </h5>
                  </div>
                </transition-group>
              </div>
            </div>
            <div v-if="gameRoomUrl" class="text-center">
              <div v-show="showRoomInvitation" style="background: #00000042">
                <hr>
                <h2 class="d-flex justify-content-end">
                  <button class="btn btn-sm btn-secondary text-white mr-3"
                      @click="toogleRoomInvitation">
                      <i class="fa-solid fa-window-minimize"></i>
                  </button>
                </h2>
                {{-- join game, invitation --}}
                <h3 class="position-relative">
                  @{{$t('game_room.invite_friends')}}
                </h3>
                <div class="row">
                  <div class="col-12">
                    <canvas id="qrcode"></canvas>
                    <h5 class="break-word">
                      <a class="text-white" :href="gameRoomUrl" target="_blank">@{{ gameRoomUrl }}</a>
                    </h5>
                    <copy-link placement="right" heading-tag="h4" custom-class="btn btn-outline-dark btn-sm text-white white-space-no-wrap" id="host-game-room-url" :url="gameRoomUrl" :text="$t('Copy')" :after-copy-text="$t('Copied link')"></copy-link>
                  </div>
                  <h4 class="col-12 mt-2">
                    @{{ $t('game_room.online_players')}}：<I-Count-Up :end-val="gameOnlineUsers"></I-Count-Up>
                  </h4>
                </div>
              </div>
            </div>

            {{-- game room setting --}}
            <div v-if="isBetGameHost" class="d-flex position-absolute mx-2 mb-4" style="right: 0; bottom:0">
              <div v-show="!showRoomInvitation"
                class="btn btn-outline-dark users-toggle"
                @click="toogleRoomInvitation">
                <h5 class="text-white align-content-center">
                  <i class="fa-solid fa-users"></i>&nbsp;<I-Count-Up :end-val="gameOnlineUsers"></I-Count-Up>
                </h5>
              </div>
              <div>
                <button v-if="!showGameRoomVotes"
                  style="min-width: 45px"
                  class="btn btn-outline-dark ml-1 white-space-no-wrap black-box-toggle" @click="toggleShowGameRoomVotes">
                  <h5>
                    <i class="fa-solid fa-box"></i>&nbsp;@{{$t('game_room.black_box')}}
                  </h5>
                </button>
                <button v-else
                  style="min-width: 45px"
                  class="btn btn-outline-dark ml-1 white-space-no-wrap black-box-toggle" @click="toggleShowGameRoomVotes">
                  <h5>
                    <i class="fa-solid fa-box-open"></i>&nbsp;@{{$t('game_room.black_box')}}
                  </h5>
                </button>
              </div>
            </div>
          </div>
          {{-- right part: ads --}}
          <div v-else class="col-12 col-xl-2">
            @if(!$post->is_censored && config('services.google_ad.enabled') && config('services.google_ad.game_page'))
              <div v-if="!refreshAD && game && !isMobileScreen" class="p-lg-1 p-xl-2">
                  @include('ads.game_ad_sides')
              </div>
            @endif
          </div>
        </div>

        {{-- 投票時間軸（桌面版顯示，手機隱藏） --}}
        <div class="row mt-3" v-if="showMatchHistory && matchHistory.length && !isMobileScreen">
          <div class="col-12">
            <div
              class="match-history-list d-flex overflow-x-scroll hide-scrollbar py-2"
              @mousedown="startDrag"
              @mousemove="onDrag"
              @mouseup="stopDrag"
              @mouseleave="stopDrag"
              style="cursor: grab;"
            >
              <div
                v-for="item in matchHistory"
                :key="item.id"
                class="match-card"
                :class="item.winSide === 'left' ? 'left-win' : 'right-win'"
              >
                {{-- Header with round info and thinking time --}}
                <div class="match-card-header d-flex justify-content-between align-items-center mb-2">
                  <div class="match-card-round font-weight-bold text-white">
                    @{{ item.roundLabel }}
                  </div>
                  <div class="match-card-progress badge badge-light">
                    @{{ item.progressLabel }}
                  </div>
                </div>

                {{-- Thinking time badge --}}
                <div class="text-center mb-2">
                  <div class="match-card-thinking badge">
                    <i class="fas fa-clock"></i> @{{ formatThinkingTime(item.thinkingTime) }}
                  </div>
                </div>

                {{-- Winner --}}
                <div class="match-card-winner mb-2 position-relative">
                  <img
                    :src="item.winner.thumb"
                    :alt="item.winner.title"
                    class="match-card-image"
                  >
                  <div class="match-card-overlay position-absolute w-100">
                    <div class="d-flex align-items-center match-card-overlay-content">
                      <i class="fas fa-trophy match-card-icon-trophy"></i>
                      <div class="match-card-title font-weight-bold text-white text-truncate">
                        @{{ item.winner.title }}
                      </div>
                    </div>
                  </div>
                </div>

                {{-- Loser --}}
                <div class="match-card-loser position-relative">
                  <img
                    :src="item.loser.thumb"
                    :alt="item.loser.title"
                    class="match-card-image"
                  >
                  <div class="match-card-overlay match-card-overlay-loser position-absolute w-100">
                    <div class="d-flex align-items-center match-card-overlay-content">
                      <i class="fas fa-times-circle match-card-icon-loss"></i>
                      <div class="match-card-title text-white text-truncate match-card-title-muted">
                        @{{ item.loser.title }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-if="game && gameSerial && !finishingGame && !isMobileScreen" class="d-flex justify-content-end py-2" id="create-game">
          <create-game-room
            game-room-route="{{route('game.room.index', '_serial')}}"
            get-room-endpoint="{{route('api.game-room.get', '_serial')}}"
            create-game-room-endpoint="{{route('api.game-room.create')}}"
            :get-game-serial="getGameSerial"
            :handle-created-room="handleCreatedRoom"
            :get-current-candidates="getCurrentCandidates"
            :has-active-room="runInBackGameRoom"
            :online-users="gameOnlineUsers"
          ></create-game-room>
        </div>


        {{-- ads at bottom --}}
        @if(!$post->is_censored && config('services.google_ad.enabled') && config('services.google_ad.game_page'))
        {{-- reserve position for ads --}}
          {{-- <div v-if="isMobileScreen" style="height: 340px; position: absolute;left:0; right:0; z-index:-1"></div> --}}
          <div v-if="isMobileScreen" id="google-ad2-container">
            <div v-if="!refreshAD && game" id="google-ad2" class="my-2 text-center position-relative">
              @include('ads.game_ad_mobile_responsive')
            </div>
          </div>
        @endif

        <div v-if="game && gameSerial && !finishingGame && isMobileScreen"
          class="position-fixed"
          id="create-game-mobile" style="right:10px; bottom:15px; z-index:1050"
          {{-- z-index:1050 to prevent modal mock in front of modal --}}
          >
          <transition-group name="slide-in-up">
            <create-game-room key="create-game-room" v-show="showCreateRoomButton"
              game-room-route="{{route('game.room.index', '_serial')}}"
              get-room-endpoint="{{route('api.game-room.get', '_serial')}}"
              create-game-room-endpoint="{{route('api.game-room.create')}}"
              :get-game-serial="getGameSerial"
              :handle-created-room="handleCreatedRoom"
              :get-current-candidates="getCurrentCandidates"
              :has-active-room="runInBackGameRoom"
              :online-users="gameOnlineUsers"
            ></create-game-room>
          </transition-group>
        </div>

        <!-- Modal, Game panel -->
        <div class="modal fade" id="gameSettingPanel" data-backdrop="static" data-keyboard="false" tabindex="-1"
          aria-labelledby="gameSettingPanelLabel" aria-hidden="true">
          <div :class="{ 'modal-dialog': true, 'modal-lg': !isMobileScreen }">
            <div class="modal-content">
              <div class="modal-header">
                <div class="d-none d-sm-flex white-space-no-wrap">
                  <h5 class="modal-title align-self-center" id="gameSettingPanelLabel">@{{ $t('game.setting') }}</h5>
                </div>
                <div class="d-flex justify-content-between w-auto">
                  <div>
                    <share-link
                      id="{{$post['serial']}}"
                      url="{{route('game.show',$post['serial'])}}"
                      text="{{__('Share')}}"
                      after-copy-text="{{__('Copied link')}}"
                      custom-class="btn btn-outline-secondary text-white mx-1 white-space-no-wrap"
                      >
                    </share-link>
                  </div>
                  <div>
                    <a class="btn btn-outline-secondary text-white white-space-no-wrap" :href="gameRankUrl">
                      <i class="fas fa-trophy"></i>&nbsp;@{{$t('game.rank')}}
                    </a>
                    <a class="btn btn-outline-secondary text-white white-space-no-wrap" href="/">
                      <i class="fas fa-home"></i>&nbsp;@{{ $t('game.cancel') }}
                    </a>
                  </div>
                </div>
              </div>
              <div class="modal-body">
                {{-- continue game --}}
                <div class="alert alert-danger" v-if="userLastGameSerial">
                  <i class="fas fa-exclamation-triangle"></i>&nbsp;@{{ $t('game.continue_hint') }}
                  <span class="btn btn-outline-danger" @click="continueGame">
                    <i class="fas fa-play"></i>&nbsp;@{{ $t('game.continue') }}
                  </span>
                </div>
                <div class="alert alert-danger" v-else-if="game_serial">
                  <i class="fas fa-exclamation-triangle"></i>&nbsp;@{{ $t('game.continue_hint') }}
                  <span class="btn btn-outline-danger" @click="continueGame">

                    <i class="fas fa-play"></i>&nbsp;@{{ $t('game.continue') }}
                  </span>
                </div>

                {{-- invalid password text --}}
                <div class="alert alert-danger" v-if="invalidPasswordWhenLoad">
                  @{{ $t('game.invalid_password') }}
                </div>

                {{-- error 404 text --}}
                <div class="alert alert-danger" v-else-if="error403WhenLoad">
                  @{{ $t('game.403') }}
                </div>

                {{-- private text --}}
                <div class="alert alert-warning" v-if="post && post.is_private">
                  @{{ $t('game.pivate_text') }}
                </div>

                {{-- password required --}}
                <div v-if="requirePassword && !post">
                  <form @submit.prevent="accessGame">
                    <div class="input-group mb-3">
                      <label class="input-group-text" for="inputPassword">@{{ $t('game.password') }}</label>
                      <input type="text" class="form-control font-size-16" v-model="inputPassword" autocomplete="off" @input="knownIncorrectPassword = false">
                      <div class="input-group-append">
                        <button class="btn btn-primary" type="submit" :disabled="inputPassword.length == 0 || knownIncorrectPassword">
                          <i class="fas fa-key"></i>&nbsp;@{{ $t('Enter') }}
                        </button>
                      </div>
                    </div>
                  </form>
                </div>

                {{-- game preview --}}
                <div class="card" v-if="post">
                  <div class="card-header text-center">
                    <h1 class="post-title">{{ $post->title }}</h1>
                  </div>

                  <div class="row no-gutters">
                    <div class="col-6">
                      <div class="post-element-container">
                        <img v-if="post.element1.previewable"
                          @@error="onImageError(post.element1.id, post.element1.url2, $event)" :src="post.element1.url"></img>
                        <video v-else :src="post.element1.url + '#t=1'"></video>
                      </div>
                      <h2 class="text-center mt-1 p-1 element-title">@{{ post.element1.title }}</h2>
                    </div>
                    <div class="col-6">
                      <div class="post-element-container">
                        <img v-if="post.element2.previewable"
                          @@error="onImageError(post.element2.id, post.element2.url2, $event)" :src="post.element2.url"></img>
                        <video v-else :src="post.element2.url + '#t=1'"></video>
                      </div>
                      <h2 class="text-center mt-1 p-1 element-title">@{{ post.element2.title }}</h2>
                    </div>
                    <div class="card-body pt-0 text-center">
                      <h5 class="text-break">{{ $post->description }}</h5>
                      <div v-if="post.tags.length > 0" class="d-flex flex-wrap">
                        <span class="badge badge-secondary m-1" v-for="tag in post.tags" style="font-size:medium">#@{{ tag }}</span>
                      </div>
                      <span class="mt-2 card-text d-flex justify-content-end">
                        <span class="pr-2">
                          <i class="fas fa-play-circle"></i>&nbsp;@{{ post.play_count }}
                        </span>
                        <small class="text-muted">@{{ post.created_at | datetime }}</small>
                      </span>
                    </div>
                  </div>
                </div>

                {{-- game setting --}}
                <div class="row mt-2" v-if="post">
                  <div class="col-12">
                    <div id="select-element-count-hint-target">
                      <div class="input-group mb-3">
                        <div class="input-group-prepend">
                          <label class="input-group-text" for="elementsCount">@{{ $t('Number of participants') }}</label>
                        </div>
                        <select v-model="elementsCount" class="custom-select" id="elementsCount" required>
                          <option value="" disabled selected="selected">@{{ $t('game.select') }}</option>
                          <option v-if="post.elements_count < 8" :value="post.elements_count">
                            @{{ post.elements_count }}
                          </option>
                          <option v-for="count in [8, 16, 32, 64, 128, 256, 512, 1024]" :value="count"
                            v-if="post.elements_count >= count">
                            @{{ count }}
                          </option>
                          <option :value="post.elements_count" v-if="!isElementsPowerOfTwo && post.elements_count > 8">
                            @{{ post.elements_count }}
                          </option>
                        </select>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class="modal-footer mb-sm-0 mb-4">
                <button v-if="post && elementsCount > 0" @click="createGame" type="submit" class="btn btn-primary btn-block" >
                  <i class="fas fa-play">&nbsp;</i>@{{ $t('game.start') }}
                </button>

                <span v-if="post && elementsCount == 0" @click="hintSelect" class="btn btn-primary disabled btn-block">
                  <i class="fas fa-play"></i>&nbsp;@{{ $t('game.start') }}
                </span>
                <b-popover :show.sync="showPopover"   ref="select-element-count-hint" target="select-element-count-hint-target" placement="top">{{ __('game.select_option_hint')}}</b-popover>
              </div>
            </div>
          </div>
        </div>

        <!-- Modal, Game panel -->
        <div class="modal fade" id="gameRoomJoin" data-backdrop="static" data-keyboard="false" tabindex="-1"
          aria-labelledby="gameRoomJoinLabel" aria-hidden="true">
          <div class="modal-dialog modal-dialog-centered" :class="{'modal-lg': !isMobileScreen }">
            <div class="modal-content ">
              <div class="modal-header">
                <h5 class="modal-title align-self-center" id="gameRoomJoinLabel">@{{$t('game_room.join_room_game_mode_preference')}}</h5>
              </div>
              <div class="modal-body">
                <h3 class="text-center">
                  @{{$t('game_room.join_room_game_mode_preference_title')}}
                </h3>
              </div>
              <div class="modal-footer mb-sm-0 mb-4">
                <button type="submit" class="btn btn-primary btn-block" @click="joinRoom">
                  <i class="fas fa-play">&nbsp;</i>@{{$t('game_room.join_room')}}
                </button>
              </div>

            </div>
          </div>
        </div>

        {{-- <transition name="fade">
          <div v-if="isCloudSaving"
              class="d-flex align-items-center bg-white shadow-sm px-3 py-2 rounded-pill"
              style="position: fixed; bottom: 20px; left: 20px; z-index: 9999; border: 1px solid #e9ecef;">

              <i class="fas fa-cloud-upload-alt text-primary mr-2 fa-fade"></i>

              <span class="text-secondary small font-weight-bold">
                  @{{ $t('game.saving_progress') }}
              </span>
          </div>
        </transition> --}}
    </div>
  </game>

@endsection
