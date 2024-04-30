@extends('layouts.app', [
    'title' => $post,
    'ogTitle' => $post->title,
    'ogImage' => $element->thumb_url,
    'ogDescription' => $post->description,
    ])

@section('header')
    <script type="application/ld+json">
    {
        "@context": "https://schema.org",
        "@type": "NewsArticle",
        "headline": "{{get_page_title($post->title)}}",
        "datePublished": "{{$post->created_at->toIso8601String()}}",
        "dateModified": "{{$post->updated_at->toIso8601String()}}"
    }
    </script>
    <script src="https://embed.twitch.tv/embed/v1.js"></script>
@endsection

@section('content')
    <game
        inline-template
        post-serial="{{$serial}}"
        get-rank-route="{{route('game.rank', '_serial')}}"
        get-game-setting-endpoint="{{route('api.game.setting', $serial)}}"
        next-round-endpoint="{{route('api.game.next-round', '_serial')}}"
        create-game-endpoint="{{route('api.game.create')}}"
        vote-game-endpoint="{{route('api.game.vote')}}"
    >
    <div class="container-fluid" v-cloak>
        <div v-if="game">
          <h2 class="text-center text-break">{{$post->title}}</h2>
          <div class="d-none d-sm-flex" style="flex-flow: row wrap">
            <h5 style="width: 20%"></h5>
            
            <h5 class="text-center align-self-center" style="width: 60%">
              <span v-if="currentRemainElement <= 2">@{{ $t('game_round_final') }}</span>
              <span v-else-if="currentRemainElement <= 4">@{{ $t('game_round_semifinal') }}</span>
              <span v-else-if="currentRemainElement <= 8">@{{ $t('game_round_quarterfinal') }}</span>
              <span v-else-if="currentRemainElement <= 1024">@{{ $t('game_round_of', {round:currentRemainElement}) }}</span>
               @{{ game.current_round }} / @{{ game.of_round }} </h5>
            <h5 class="text-right align-self-center" style="width: 20%">(@{{ game.remain_elements }} /@{{ game.total_elements }})</h5>
          </div>
        </div>
        <div class="row game-body" v-if="game">
          <!--left part-->
          <div class="col-sm-12 col-md-6 pr-md-1 mb-2 mb-md-0">
            <div class="card game-player left-player" id="left-player">
              <div v-show="isImageSource(le)" class="game-image-container" v-cloak>
                <div v-show="!leftImageLoaded" class="text-center align-content-center" :style="{ height: this.elementHeight + 'px' }">
                  <i class="fas fa-3x fa-spinner fa-spin" ></i>
                </div>
                <img @load="handleLeftLoaded" v-show="leftImageLoaded" @click="clickImage" @@error="onImageError(le.id, le.thumb_url2,$event)" class="game-image" :src="le.thumb_url"
                  :style="{ height: this.elementHeight + 'px' }">
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
                <video width="100%" :height="elementHeight" loop autoplay muted controls playsinline :src="le.thumb_url"></video>
              </div>
              <div class="card-body text-center">
                <div class="my-1" style="max-height: 120px" v-if="isMobileScreen">
                  <h5 class="my-1 font-size-small">@{{ le.title }}</h5>
                </div>
                <div class="my-1" style="height: 120px" v-else>
                  <h5 class="my-1">@{{ le.title }}</h5>
                </div>
                <button id="left-btn" class="btn btn-primary vote-button btn-block d-none d-md-block" :disabled="isVoting"
                  @click="leftWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                </button>
                <div class="row" v-if="isYoutubeSource(le)">
                  {{-- mobile --}}
                  <div class="col-3">
                    <button class="btn btn-outline-primary btn-lg btn-block d-block d-md-none" :class="{active: isLeftPlaying}" :disabled="isVoting"
                      @click="leftPlay()">
                      <i class="fas fa-volume-mute" v-show="!isLeftPlaying"></i>
                      <i class="fas fa-volume-up fa-beat" v-show="isLeftPlaying"></i>
                    </button>
                  </div>
                  <div class="col-9">
                    <button class="btn btn-primary btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                      @click="leftWin"><i class="fa-solid fa-thumbs-up"></i>
                    </button>
                  </div>
                </div>
                <div v-else>
                  <button class="btn btn-primary btn-block btn-lg d-block d-md-none" :disabled="isVoting"
                    @click="leftWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                  </button>
                </div>
              </div>
            </div>
          </div>
    
          <!-- mobile rounds session -->
          <div id="rounds-session" class="col-sm-12 d-md-none">
            <div class="d-flex d-sm-none justify-content-between" style="flex-flow: row wrap">
              <h5 class="">
                <span v-if="currentRemainElement <= 2">@{{ $t('game_round_final') }}</span>
                <span v-else-if="currentRemainElement <= 4">@{{ $t('game_round_semifinal') }}</span>
                <span v-else-if="currentRemainElement <= 8">@{{ $t('game_round_quarterfinal') }}</span>
                <span v-else-if="currentRemainElement <= 1024">@{{ $t('game_round_of', {round:currentRemainElement}) }}</span>
                   @{{ game.current_round }} / @{{ game.of_round }} </h5>
              <h5 class="">(@{{ game.remain_elements }} /@{{ game.total_elements }})</h5>
            </div>
          </div>
    
          <!--right part-->
          <div class="col-sm-12 col-md-6 pl-md-1 mb-4 mb-md-0">
            <div class="card game-player right-player" :class="{ 'flex-column-reverse': isMobileScreen }" id="right-player">
              <div v-show="isImageSource(re)" class="game-image-container" v-cloak>
                <div v-show="!rightImageLoaded" class="text-center align-content-center" :style="{ height: this.elementHeight + 'px' }">
                  <i class="fas fa-3x fa-spinner fa-spin" ></i>
                </div>
                <img @load="handleRightLoaded" v-show="rightImageLoaded" @click="clickImage" @@error="onImageError(re.id, re.thumb_url2, $event)" class="game-image" :src="re.thumb_url"
                  :style="{ height: this.elementHeight + 'px' }">
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
                <video width="100%" :height="elementHeight" loop autoplay controls muted playsinline :src="re.thumb_url" ></video>
              </div>
    
              <!-- reverse when device size width less md(768px)-->
              <div class="card-body text-center"
                :class="{ 'flex-column-reverse': isMobileScreen, 'd-flex': isMobileScreen }">
                <div class="my-1 flex-column-reverse d-flex" style="max-height: 120px" v-if="isMobileScreen">
                  <h5 class="my-1 font-size-small">@{{ re.title }}</h5>
                </div>
                <div class="my-1" style="height: 120px" v-else>
                  <h5 class="my-1">@{{ re.title }}</h5>
                </div>
                <button id="right-btn" class="btn btn-danger vote-button btn-block d-none d-md-block" :disabled="isVoting"
                  @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                </button>
                <div class="row" v-if="isYoutubeSource(re)">
                  {{-- mobile --}}
                  <div class="col-3">
                    <button class="btn btn-outline-danger btn-lg btn-block d-block d-md-none" :class="{active: isRightPlaying}" :disabled="isVoting"
                      @click="rightPlay()">
                      <i class="fas fa-volume-mute" v-show="!isRightPlaying"></i>
                      <i class="fas fa-volume-up fa-beat" v-show="isRightPlaying"></i>
                    </button>
                  </div>
                  <div class="col-9">
                    <button class="btn btn-danger btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                      @click="rightWin"><i class="fa-solid fa-thumbs-up"></i>
                    </button>
                  </div>
                </div>
                <div v-else>
                  <button class="btn btn-danger btn-lg btn-block d-block d-md-none" :disabled="isVoting"
                    @click="rightWin"><i class="fa-solid fa-2x fa-thumbs-up"></i>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        @if(config('services.google_ad.enabled') && config('services.google_ad.game_page'))
          <div v-if="!refreshAD && game && !isMobileScreen" id="google-ad" class="my-2">
              @include('ads.game_ad')
          </div>
          <div v-if="!refreshAD && game && isMobileScreen" id="google-ad2" class="my-2 text-center">
            @include('ads.game_ad_mobile')
          </div>
        @endif

        <!-- Modal -->
        <div class="modal fade" id="gameSettingPanel" data-backdrop="static" data-keyboard="false" tabindex="-1"
          aria-labelledby="gameSettingPanelLabel" aria-hidden="true">
          <div :class="{ 'modal-dialog': true, 'modal-lg': !isMobileScreen }">
            <div class="modal-content">
              <div class="modal-header">
                <h5 class="modal-title align-self-center" id="gameSettingPanelLabel">@{{ $t('game.setting') }}</h5>
                <div>
                  <a class="btn btn-outline-secondary" :href="gameRankUrl">
                    @{{$t('game.rank')}}&nbsp;<i class="fas fa-trophy"></i>
                  </a>
                  <a class="btn btn-outline-secondary" href="/">
                    @{{ $t('game.cancel') }}&nbsp;<i class="fas fa-times"></i>
                  </a>
                </div>
              </div>
              <div class="modal-body">
                <div class="alert alert-danger" v-if="processingGameSerial">
                  <i class="fas fa-exclamation-triangle"></i>@{{ $t('game.continue_hint') }}
                  <span class="btn btn-outline-danger" @click="continueGame">
        
                    @{{ $t('game.continue') }}&nbsp;<i class="fas fa-play"></i>
                  </span>
                </div>
                <div class="alert alert-danger" v-if="error403WhenLoad">
                  @{{ $t('game.403') }}
                </div>
                <div class="alert alert-warning" v-if="post && post.is_private">
                  @{{ $t('game.pivate_text') }}
                </div>
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
                <div class="row mt-2" v-if="post">
                  <div class="col-12">
                    <div id="select-element-count-hint-target">
                      <div class="input-group mb-3">
                        <div class="input-group-prepend">
                          <label class="input-group-text" for="elementsCount">@{{ $t('Number of participants') }}</label>
                        </div>
                        <select v-model="elementsCount" class="custom-select" id="elementsCount" required>
                          <option value="" disabled selected="selected">@{{ $t('game.select') }}</option>
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
                  @{{ $t('game.start') }}&nbsp;<i class="fas fa-play"></i>
                </button>
                
                <span v-if="post && elementsCount == 0" @click="hintSelect" class="btn btn-primary disabled btn-block">
                  @{{ $t('game.start') }}&nbsp;<i class="fas fa-play"></i>
                </span>
                <b-popover :show.sync="showPopover"   ref="select-element-count-hint" target="select-element-count-hint-target" placement="top">{{ __('game.select_option_hint')}}</b-popover>
              </div>
            </div>
          </div>
        </div>

        
    </div>
  </game>
@endsection
