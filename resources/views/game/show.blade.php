@extends('layouts.app', [
    'title' => $post->isPublic() ? $post : __('title.access'),
    'ogTitle' => $post->isPublic() ? $post->title : null,
    'ogImage' => $post->isPublic() ? $element?->getScaledThumbUrl() : null,
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
@endsection

@section('content')
    <game
        inline-template
        post-serial="{{$serial}}"
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
        update-room-profile-endpoint="{{route('api.game-room.update-profile', '_serial')}}"
        bet-endpoint="{{route('api.game-room.bet', '_serial')}}"
    >
    <div class="container-fluid hide-scrollbar" v-cloak>
        @if(config('services.google_ad.enabled') && config('services.google_ad.game_page'))
        {{-- ads --}}
          <div v-if="!isMobileScreen" style="height: 100px">
            <div v-if="!refreshAD && game" id="google-ad" class="my-2 text-center">
                @include('ads.game_ad_pc')
            </div>
          </div>
          <div v-if="isMobileScreen" id="google-ad-container" style="height: 100px; z-index:-1" class="overflow-hidden position-relative">
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
        <div v-show="finishingGame && !isGameClient">
          <div class="d-flex justify-content-center align-items-center flex-column" style="height: 100vh">
            <img src="{{ asset('storage/logo.png') }}" class="updown-animation" alt="logo" style="width: 50px; height: 50px;">
            <div v-if="gameResultUrl === ''">
              @{{ $t('game.finishing') }}
              {{-- spinner --}}
              <div class="spinner-border text-primary" role="status">
                <span class="sr-only">Loading...</span>
              </div>
            </div>
            <div v-else>
              <a :href="gameResultUrl">@{{ $t('game.checkout_result') }}</a>
            </div>
          </div>
        </div>

        {{-- main --}}
        <div class="row">
          {{-- elements --}}
          <div :class="{'col-12 col-xl-9':gameRoomSerial, 'col-12': !gameRoomSerial}">

            {{-- bet success animation: firework --}}
            <div class="pyro" v-if="showFirework">
              <div class="before" style="z-index: 110"></div>
              <div class="after" style="z-index: 110" ></div>
            </div>
            <transition-group name="bet-result-animation" tag="div" class="position-absolute top-50 left-50 translate-50-50 w-100 bg-dark-50 overflow-hidden" style="z-index: 110">
              <div v-if="showFirework" key="bet-success">
                <h1 class="text-white text-center">預測成功</h1>
              </div>
            </transition-group>
            <transition-group name="bet-result-animation" tag="div" class="position-absolute top-50 left-50 translate-50-50 w-100 bg-dark-50 overflow-hidden" style="z-index: 110">
              <div v-if="showBetFailed" key="bet-success">
                <h1 class="text-white text-center">預測失敗</h1>
              </div>
            </transition-group>

            <div v-if="gameRoom && gameRoom.is_game_completed">
              <div class="d-flex justify-content-center align-content-center mt-4">
                <h3>投票已結束</h3>
              </div>
            </div>

            {{-- game heading --}}
            <div v-if="game && !finishingGame">
              <h1 id="game-title" class="text-center text-break mt-1">{{$post->title}}</h1>
              <div class="d-none d-sm-flex" style="flex-flow: row wrap">
                <h3 style="width: 20%">
                  <span v-if="isGameClient && !isVoting">預測誰會勝出？</span>
                  <span v-if="isGameClient && isVoting">等待結果中...</span>
                </h3>

                <h3 class="text-center align-self-center" style="width: 60%">
                  <span v-if="currentRemainElement <= 2">@{{ $t('game_round_final') }}</span>
                  <span v-else-if="currentRemainElement <= 4">@{{ $t('game_round_semifinal') }}</span>
                  <span v-else-if="currentRemainElement <= 8">@{{ $t('game_round_quarterfinal') }}</span>
                  <span v-else-if="currentRemainElement <= 1024">@{{ $t('game_round_of', {round:currentRemainElement}) }}</span>
                  @{{ game.current_round }} / @{{ game.of_round }}
                </h3>

                <h3 class="text-right align-self-center" style="width: 20%">(@{{ game.remain_elements }} /@{{ game.total_elements }})</h3>
              </div>
            </div>

            {{-- playground --}}
            <div class="row overflow-hidden" v-if="game && !finishingGame && (! gameRoom || !gameRoom.is_game_completed)" :style="{'height': this.gameBodyHeight + 'px'}">
              <!--left part-->
              <div class="col-12 col-sm-6 pr-sm-1 mb-2 mb-sm-0">
                <transition name="slide-in-left">
                  <div v-if="animationShowLeftPlayer" class="card game-player left-player" id="left-player">
                    <div v-show="isImageSource(le)" class="game-image-container" v-cloak>
                      <div v-show="!leftImageLoaded" class="text-center align-content-center" :style="{ height: this.elementHeight + 'px' }">
                        <i class="fas fa-3x fa-spinner fa-spin" ></i>
                      </div>
                      <viewer ref="leftViewer" :options="viewerOptions">
                        <img
                          v-show="leftImageLoaded"
                          @load="handleLeftLoaded"
                          @@error="onImageError(le.id, le.thumb_url2,$event)"
                          :src="getThumbUrl(le)"
                          :style="{ height: this.elementHeight + 'px' }"
                          :key="le.thumb_url">
                      </viewer>
                    </div>
                    <div v-if="isYoutubeSource(le) && !isDataLoading" class="d-flex" @mouseover="videoHoverIn(le, re, true)">
                      <youtube :video-id="le.video_id" width="100%" :height="elementHeight" :ref="le.id"
                        :player-vars="{ controls: 1, autoplay: !isMobileScreen, rel: 0 , origin: origin, playlist: le.video_id, start:le.video_start_second, end:le.video_end_second }">
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
                      <video @mouseover="videoHoverIn(le, re, true)" id="left-video-player" v-else width="100%" :height="elementHeight" loop autoplay controls playsinline muted :src="le.source_url" :poster="le.thumb_url"></video>
                    </div>
                    <div class="card-body text-center">
                      <div class="my-1 overflow-scroll hide-scrollbar" style="max-height: 35px" v-if="isMobileScreen">
                        <h1 class="my-1 font-size-small">@{{ le.title }}</h1>
                      </div>
                      <div class="my-1 overflow-scroll hide-scrollbar" style="height: 120px" v-else>
                        <h1 class="my-1 game-element-title">@{{ le.title }}</h1>
                      </div>
                      <button id="left-btn" class="btn btn-primary vote-button btn-block d-none d-sm-block" :disabled="isVoting"
                        @click="leftWin">
                        <i class="fa-solid fa-2x fa-thumbs-up"></i>
                        <h3 class="d-inline-block m-0" v-if="showGameRoomVotes">
                          <I-Count-Up :end-val="leftVotes"></I-Count-Up>
                          (<I-Count-Up :end-val="leftVotesPercentage"></I-Count-Up>%)
                        </h3>
                      </button>
                      <div class="row" v-if="isYoutubeSource(le) || isVideoUrlSource(le)">
                        {{-- mobile --}}
                        <div class="col-3">
                          <button class="btn btn-outline-primary btn-lg btn-block d-block d-sm-none" :class="{active: isLeftPlaying}" :disabled="isVoting"
                            @click="leftPlay()">
                            <i class="fas fa-volume-mute" v-show="!isLeftPlaying"></i>
                            <i class="fas fa-volume-up" v-show="isLeftPlaying"></i>
                          </button>
                        </div>
                        <div class="col-9">
                          <button class="btn btn-primary btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                            @click="leftWin"><i class="fa-solid fa-thumbs-up"></i>
                            <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                              <I-Count-Up :end-val="leftVotes"></I-Count-Up>
                              (<I-Count-Up :end-val="leftVotesPercentage"></I-Count-Up>%)
                            </p>
                          </button>
                        </div>
                      </div>

                      <div v-else>
                        <button class="btn btn-primary btn-block btn-lg d-block d-md-none" :disabled="isVoting"
                          @click="leftWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                          <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                            <I-Count-Up :end-val="leftVotes"></I-Count-Up>
                            (<I-Count-Up :end-val="leftVotesPercentage"></I-Count-Up>%)
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
                      <span v-if="currentRemainElement <= 2">@{{ $t('game_round_final') }}</span>
                      <span v-else-if="currentRemainElement <= 4">@{{ $t('game_round_semifinal') }}</span>
                      <span v-else-if="currentRemainElement <= 8">@{{ $t('game_round_quarterfinal') }}</span>
                      <span v-else-if="currentRemainElement <= 1024">@{{ $t('game_round_of', {round:currentRemainElement}) }}</span>
                        @{{ game.current_round }} / @{{ game.of_round }}
                    </h5>
                    <h5 v-if="isGameClient">
                      <span v-if="isGameClient && !isVoting">預測誰會勝出？</span>
                      <span v-if="isGameClient && isVoting">等待結果中...</span>
                    </h5>
                    <h5 class="">(@{{ game.remain_elements }} /@{{ game.total_elements }})</h5>

                  </div>
                </div>
              </transition>

              <!--right part-->
              <div class="col-12 col-sm-6 pl-sm-1 mb-4 mb-sm-0">
                <transition :name="isMobileScreen ? 'slide-in-left' : 'slide-in-right'">
                  <div v-if="animationShowRightPlayer" class="card game-player right-player" id="right-player" :class="{ 'flex-column-reverse': isMobileScreen, 'mb-4': isMobileScreen}">
                    <div v-show="isImageSource(re)" class="game-image-container" v-cloak>
                      <div v-show="!rightImageLoaded" class="text-center align-content-center" :style="{ height: this.elementHeight + 'px' }">
                        <i class="fas fa-3x fa-spinner fa-spin" ></i>
                      </div>
                      <viewer ref="rightViewer" :options="viewerOptions">
                        <img
                          v-show="rightImageLoaded"
                          @load="handleRightLoaded"
                          @@error="onImageError(re.id, re.thumb_url2, $event)"
                          :src="getThumbUrl(re)"
                          :style="{ height: this.elementHeight + 'px' }" :key="re.thumb_url">
                      </viewer>
                    </div>
                    <div v-if="isYoutubeSource(re) && !isDataLoading" class="d-flex" @mouseover="videoHoverIn(re, le, false)">
                      <youtube :video-id="re.video_id" width="100%" :height="elementHeight" :ref="re.id"
                        :player-vars="{ controls: 1, autoplay: !isMobileScreen, rel: 0, origin: origin,  playlist: re.video_id, start:re.video_start_second, end:re.video_end_second}">
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
                      <video @mouseover="videoHoverIn(re, le, false)" id="right-video-player" v-else width="100%" :height="elementHeight" loop autoplay controls playsinline muted :src="re.source_url" :poster="re.thumb_url"></video>
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
                      <button id="right-btn" class="btn btn-danger vote-button btn-block d-none d-sm-block" :disabled="isVoting"
                        @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                        <h3 class="d-inline-block m-0" v-if="showGameRoomVotes">
                          <I-Count-Up :end-val="rightVotes"></I-Count-Up>
                          (<I-Count-Up :end-val="rightVotesPercentage"></I-Count-Up>%)
                        </h3>
                      </button>
                      <div class="row" v-if="isYoutubeSource(re) || isVideoUrlSource(re)">
                        {{-- mobile --}}
                        <div class="col-3">
                          <button class="btn btn-outline-danger btn-lg btn-block d-block d-sm-none" :class="{active: isRightPlaying}" :disabled="isVoting"
                            @click="rightPlay()">
                            <i class="fas fa-volume-mute" v-show="!isRightPlaying"></i>
                            <i class="fas fa-volume-up" v-show="isRightPlaying"></i>
                          </button>
                        </div>
                        <div class="col-9">
                          <button class="btn btn-danger btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                            @click="rightWin"><i class="fa-solid fa-thumbs-up"></i>
                            <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                              <I-Count-Up :end-val="rightVotes"></I-Count-Up>
                              (<I-Count-Up :end-val="rightVotesPercentage"></I-Count-Up>%)
                            </p>
                          </button>
                        </div>
                      </div>
                      <div v-else>
                        <button class="btn btn-danger btn-lg btn-block d-block d-sm-none" :disabled="isVoting"
                          @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                          <p class="d-inline-block m-0" v-if="showGameRoomVotes">
                            <I-Count-Up :end-val="rightVotes"></I-Count-Up>
                            (<I-Count-Up :end-val="rightVotesPercentage"></I-Count-Up>%)
                          </p>
                        </button>
                      </div>
                    </div>
                  </div>
                </transition>
              </div>

            </div>
          </div>


          {{-- game room --}}
          <div v-if="gameRoom" class="col-12 col-xl-3 bg-secondary text-white mt-2 mt-xl-0 game-room-box position-relative">
            <div class="game-room-container">
              {{-- game room --}}
              <div class="p-1 my-1" v-if="isGameClient">
                <input v-show="isEditingNickname" id="newNickname" autocomplete="off" v-model="newNickname" type="text" class="form-control badge-secondary bg-secondary-onfocus" maxlength="10">
                <div class="text-center" v-show="isEditingNickname">(上限10個字元，每小時可更改一次)</div>
                <h4 class="d-flex justify-content-center">
                  <div class="position-relative">
                    <span v-if="!isEditingNickname">@{{gameRoom.user.name | cut(10)}}</span>
                    {{-- edit name --}}
                    <button v-if="!isEditingNickname && !gameRoom.is_game_completed" class="btn btn-secondary btn-sm position-absolute m-0 p-0 px-1" @click="toggleEditNickname">
                      <i class="fa-solid fa-pen-to-square"></i>
                    </button>
                    <div v-if="isEditingNickname && !gameRoom.is_game_completed">
                      <button class="btn btn-secondary btn-sm m-0 p-0 px-1" @click="saveNickname">
                        <i class="fa-solid fa-check"></i>
                      </button>
                      <button class="btn btn-secondary btn-sm m-0 p-0 px-1" @click="toggleEditNickname">
                        <i class="fa-solid fa-xmark"></i>
                      </button>
                    </div>
                  </div>
                </h4>
                <div class="d-flex justify-content-between">
                  <span>積分:</span>
                  <div>
                    <span data-toggle="tooltip" data-placement="top" :title="$t('game.bet.combo',{number: gameRoom.user.combo})">
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
                  <span>排名 / 人數:</span>
                  <span v-if="gameRoom.user.rank > 0">
                    <I-Count-Up :end-val="gameRoom.user.rank"></I-Count-Up>
                    <span>/</span>
                    <I-Count-Up :end-val="gameRoom.total_users"></I-Count-Up>
                  </span>
                  <span v-if="gameRoom.user.rank == 0">無 / @{{gameRoom.total_users}}</span>
                </div>
                <div class="d-flex justify-content-between">
                  <span>成功率:</span>
                  <span data-toggle="tooltip" data-placement="left" :title="gameRoom.user.total_correct+' / '+gameRoom.user.total_played">
                    <I-Count-Up :end-val="Number(gameRoom.user.accuracy)" :options="{suffix:'%', decimalPlaces:'2'}"></I-Count-Up>
                  </span>
                </div>
              </div>
              {{-- rank --}}
              <div class="p-1 my-1">
                <h3 class="text-center">排行榜</h3>
                <h5 class="d-flex justify-content-between bet-rank-broad">
                  <div class="d-flex align-items-center">
                    <span class="badge badge-pill badge-light mr-1">
                    #
                    </span>
                    暱稱
                  </div>
                  <span>積分
                    <i class="fa-solid fa-circle-question" data-toggle="tooltip" data-placement="left" data-html="true" title="預測正確:連勝*10分<br>預測失敗:-10分"></i>
                  </span>
                </h5>
                <hr class="my-1 text-white bg-white">
                <transition-group name="list-left" tag="div">
                  <div class="d-flex justify-content-between position-relative align-items-center" v-if="gameBetRanks.top_10" v-for="(rank,index) in gameBetRanks.top_10" :key="rank.user_id+':'+rank.rank">
                    <h5 class="d-flex align-items-center bet-rank-broad">
                      <span class="badge badge-pill badge-light mr-1" v-if="rank.rank">
                        @{{rank.rank}}
                      </span>
                      <span :class="{'text-warning':isSameUser(rank)}">
                        @{{rank.name | cut(10)}}
                      </span>
                    </h5>
                    <h5 class="position-absolute bet-rank-broad" style="right: 0">
                      <span data-toggle="tooltip" data-placement="top" :title="$t('game.bet.combo',{number:rank.combo})">
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
            <div v-if="gameRoomUrl" class="text-center">
              <div v-show="showRoomInvitation">
                <hr>
                {{-- join game, invitation --}}
                <h2 class="position-relative">
                  加入遊戲
                  <button class="btn btn-sm btn-outline-dark position-absolute text-white"
                      style="top: 0; right: 0"
                      @click="toogleRoomInvitation"
                      data-toggle="tooltip" data-placement="top" title="縮小">
                    <i class="fa-regular fa-square-minus"></i>
                  </button>
                </h2>
                <div class="row">
                  <div class="col-12">
                    <canvas id="qrcode"></canvas>
                    <h5 class="break-word">
                      <a class="text-white" :href="gameRoomUrl" target="_blank">@{{ gameRoomUrl }}</a>
                    </h5>
                    <copy-link heading-tag="h4" custom-class="btn btn-outline-dark btn-sm text-white" id="host-game-room-url" :url="gameRoomUrl" :text="$t('Copy')" :after-copy-text="$t('Copied link')"></copy-link>
                  </div>
                  <h3 class="col-12 mt-2">
                    在線人數：<I-Count-Up :end-val="gameRoom.total_users"></I-Count-Up>
                  </h3>
                </div>
              </div>
            </div>
            <div v-show="!showRoomInvitation"
              class="position-absolute btn btnsm btn-outline-dark mr-1 mb-1"
              data-toggle="tooltip" data-placement="top" title="展開"
              style="right: 0; bottom:0"
              @click="toogleRoomInvitation">
              <div class="text-white">
                在線人數：<I-Count-Up :end-val="gameRoom.total_users"></I-Count-Up>
              </div>
            </div>

          </div>
        </div>
        <div v-if="gameSerial" class="d-flex justify-content-end my-2">
          <create-game-room
            game-room-route="{{route('game.room.index', '_serial')}}"
            get-room-endpoint="{{route('api.game-room.get', '_serial')}}"
            create-game-room-endpoint="{{route('api.game-room.create')}}"
            :get-game-serial="getGameSerial"
            :handle-created-room="handleCreatedRoom"
          ></create-game-room>
          <div v-if="gameRoom" id="game-room-votes">
            <button v-if="!showGameRoomVotes"
              v-b-tooltip.hover="'打開黑箱'"
              style="min-width: 45px"
              class="btn btn-outline-secondary mx-1" @click="toggleShowGameRoomVotes">
              <i class="fa-solid fa-box text-dark"></i>&nbsp;黑箱
            </button>
            <button v-else
              v-b-tooltip.hover="'關閉黑箱'"
              style="min-width: 45px"
              class="btn btn-outline-secondary mx-1" @click="toggleShowGameRoomVotes">
              <i class="fa-solid fa-box-open text-dark"></i>&nbsp;黑箱
            </button>
          </div>
        </div>

        {{-- ads at bottom --}}
        @if(config('services.google_ad.enabled') && config('services.google_ad.game_page'))
        {{-- reserve position for ads --}}
          <div style="height: 380px; position: absolute;left:0; right:0; z-index:-1"></div>
          <div v-if="!isMobileScreen" id="google-ad2-container">
            <div v-if="!refreshAD && game" id="google-ad2" class="my-2 text-center">
                @include('ads.game_ad_pc_responsive')
            </div>
          </div>
          <div v-if="isMobileScreen" id="google-ad2-container">
            <div v-if="!refreshAD && game" id="google-ad2" class="my-2 text-center position-relative">
              @include('ads.game_ad_mobile_responsive')
            </div>
          </div>
        @endif


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
                      custom-class="btn btn-outline-secondary mx-1 white-space-no-wrap"
                      >
                    </share-link>
                  </div>
                  <div>
                    <a class="btn btn-outline-secondary white-space-no-wrap" :href="gameRankUrl">
                      <i class="fas fa-trophy"></i>&nbsp;@{{$t('game.rank')}}
                    </a>
                    <a class="btn btn-outline-secondary white-space-no-wrap" href="/">
                      <i class="fas fa-home"></i>&nbsp;@{{ $t('game.cancel') }}
                    </a>
                  </div>
                </div>
              </div>
              <div class="modal-body">
                {{-- continue game --}}
                <div class="alert alert-danger" v-if="game_serial">
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
                      <input type="text" class="form-control" v-model="inputPassword" autocomplete="off" @input="knownIncorrectPassword = false">
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
                        <span class="badge badge-secondary m-1" v-for="tag in post.tags" style="font-size:medium">#@{{ tag}}</span>
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
                          <option :value="post.elements_count" v-if="!isElementsPowerOfTwo">
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

    </div>
  </game>

@endsection
