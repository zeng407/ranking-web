@extends('layouts.app', [
  'ogImage' => asset('/storage/og-image.jpeg'),
  'stickyNav' => 'sticky-top'
  ])

@section('content')
    <home
      inline-template
      sort-by="{{$sort ?? 'hot'}}"
      range="{{$range ?? config('setting.home_page_default_range')}}"
      keyword="{{Request::get('k') }}"
      :current-page={{$posts->currentPage()}}
      index-posts-endpoint="{{route('api.public-post.index')}}"
      show-game-endpoint="{{route('game.show', '_serial')}}"
      show-rank-endpoint="{{route('game.rank', '_serial')}}"
      get-champions-endpoint="{{route('api.champion.index')}}"
    >
    <div class="container-fuild" v-cloak>
      @include('partial.lang')
      <div class="row m-0">
        {{-- left part: champions --}}
        <div class="d-none d-md-block col-md-2 p0">
          <div id="champions-region" class="container position-sticky hide-scrollbar champions-container">
            {{-- google ads --}}
            <div id="champion-ad-top">
              @include('ads.home_ad_champion_top', ['id' => 'champion-ad-top'])
            </div>

            <div v-if="champions.length" v-cloak>
              <h4 class="text-center my-1">@{{$t('home.new_champions')}}</h4>
              <hr>
              <div class="position-relative" v-for="championResult in champions">
                <div class="text-center"><a target="_blank" :href="getShowGameUrl(championResult.post_serial)">@{{championResult.post_title}}</a></div>
                <div class="row">
                  <div class="col-6 pr-0">
                    <div class="position-relative">
                      {{-- if championResult.left.thumb_url is end with mp4 --}}
                      <video v-if="isEndWith(championResult.left.thumb_url, 'mp4')" @loadeddata="handleCandicateLoaded(championResult.left)" v-show="!isChampionLoading(championResult.left)" class="bg-dark champion-card w-100" :class="{'eliminated-image': !championResult.left.is_winner}" :src="championResult.left.thumb_url + '#t=0.01'"  muted></video>
                      <img v-else @load="handleCandicateLoaded(championResult.left)" v-show="!isChampionLoading(championResult.left)" class="bg-dark champion-card w-100" :class="{'eliminated-image': !championResult.left.is_winner}" :src="championResult.left.thumb_url">
                      <div v-show="isChampionLoading(championResult.left)" class="champion-card">
                        <div class="position-absolute w-100 h-100 bg-dark d-flex justify-content-center align-items-center">
                          <i class="fas fa-spinner fa-spin fa-2x text-white"></i>
                        </div>
                      </div>
                      <i class="fa-solid fa-5x fa-x eliminated-x" v-if="!championResult.left.is_winner" v-show="!isChampionLoading(championResult.left)"></i>
                    </div>
                    <h5 class="text-center font-size-small">@{{championResult.left.name}}</h5> 
                  </div>
                  <div class="col-6 pl-0" v-if="championResult.right.name">
                    <div class="position-relative">
                      <video  v-if="isEndWith(championResult.right.thumb_url, 'mp4')" @loadeddata="handleCandicateLoaded(championResult.right)" v-show="!isChampionLoading(championResult.right)" class="bg-dark champion-card w-100" :class="{'eliminated-image': !championResult.right.is_winner}" :src="championResult.right.thumb_url + '#t=0.01'"  muted></video>
                      <img v-else @load="handleCandicateLoaded(championResult.right)" v-show="!isChampionLoading(championResult.right)" class="bg-dark champion-card w-100" :class="{'eliminated-image': !championResult.right.is_winner}" :src="championResult.right.thumb_url">
                      <div  v-show="isChampionLoading(championResult.right)" class="champion-card">
                        <div class="position-absolute w-100 h-100 bg-dark d-flex justify-content-center align-items-center">
                          <i class="fas fa-spinner fa-spin fa-2x text-white"></i>
                        </div>
                      </div>
                      <i class="fa-solid fa-5x fa-x eliminated-x" v-if="!championResult.right.is_winner" v-show="!isChampionLoading(championResult.right)"></i>
                    </div>
                    <h5 class="text-center font-size-small">@{{championResult.right.name}}</h5> 
                  </div>
                </div>
                <p  :key="refreshKey" class="text-right font-size-small">@{{humanizeDate(championResult.datetime)}}</p>
                <hr>
              </div>
            </div>

            {{-- ads --}}
            <div id="home_ad_champion_bottom">
              @include('ads.home_ad_champion_bottom', ['id'=>'home_ad_champion_bottom'])
            </div>
          </div>
        </div>
        {{-- right part: posts --}}
        <div id="main-region" class="col-12 col-md-10">
          @if(empty(Request::all()))
            @include('partial.home-carousel')
          @else
            {{-- ads --}}
            <div id="home-ad-top">
              @include('ads.home_ad_top', ['id' => 'home-ad-top'])
            </div>
          @endif
          {{-- sorter --}}
          <div class="d-flex justify-content-between flex-nowrap mt-4">
            <div class="form-inline">
              <div class="btn-group btn-group-toggle" data-toggle="buttons">
      
                <label class="btn btn-outline-dark mr-2">
                  <input type="radio" v-model="filters.sort_by" value="hot">
                  <i class="fas fa-fire-alt"></i>&nbsp;@{{ $t('Hot') }}
                </label>
                <label class="btn btn-outline-dark mr-2">
                  <input type="radio" v-model="filters.sort_by" value="new">
                  <i class="fas fa-sort-amount-down-alt"></i>&nbsp;@{{ $t('New') }}
                </label>
              </div>
            </div>
          </div>
          <div class="d-flex justify-content-start" v-if="timeRangeText">
            <div class="form-inline mt-3">
              <button type="button" class="btn btn-outline-dark btn-sm rounded-pill" data-toggle="dropdown"
                v-show="filters.sort_by === 'hot'" style="top: 0">
                @{{ timeRangeText }}
                <i class="fas fa-caret-down"></i>
              </button>
              <div class="dropdown-menu">
                <span class="dropdown-item" @click="clickTimeRange($event, 'all')" >@{{ $t('All Time') }}</span>
                <span class="dropdown-item" @click="clickTimeRange($event, 'year')" >@{{ $t('This Year') }}</span>
                <span class="dropdown-item" @click="clickTimeRange($event, 'month')" >@{{ $t('This Month') }}</span>
                <span class="dropdown-item" @click="clickTimeRange($event, 'week')" >@{{ $t('This Week') }}</span>
                <span class="dropdown-item" @click="clickTimeRange($event, 'day')" >@{{ $t('Today') }}</span>
              </div>
            </div>
          </div>
      
          {{-- preload posts --}}
          <div class="grid pt-4">
            @foreach($posts as $index => $post)
            <div class="grid-sizer"></div>
            <div class="gutter-sizer"></div>
            <div class="grid-item pt-2">
              <div class="card shadow">
                <div class="card-header text-center">
                  <h2 class="post-title">{{ $post['title'] }}</h2>
                </div>
                <div class="row no-gutters">
                  <div class="col-6">
                    <div class="post-element-container">
                      @if($post['element1']['previewable'])
                        <img src="{{$post['element1']['url']}}" @@error="onImageError('{{$post['element1']['url2']}}', $event)">
                      @else
                        <video src="{{$post['element1']['url']}}#t=0.01"></video>
                      @endif
                    </div>
                    <h3 class="text-center mt-1 p-1 element-title">{{ $post['element1']['title'] }}</h3>
                  </div>
                  <div class="col-6">
                    <div class="post-element-container">
                      @if($post['element2']['previewable'])
                        <img src="{{$post['element2']['url']}}" @@error="onImageError('{{$post['element2']['url2']}}', $event)">
                      @else
                        <video src="{{$post['element2']['url']}}#t=0.01"></video>
                      @endif
                    </div>
                    <h5 class="text-center mt-1 p-1">{{ $post['element2']['title'] }}</h5>
                  </div>
                  <div class="card-body pt-0 text-center">
                    <p class="text-break">{{ $post['description'] }}</p>
                    <div class="row">
                      <div class="col-6">
                        <a class="btn btn-primary btn-block" href="{{route('game.show', $post['serial'])}}" target="_blank">
                          <i class="fas fa-play"></i> {{__('home.start')}}
                        </a>
                      </div>
                      <div class="col-6">
                        <a class="btn btn-secondary btn-block" href="{{route('game.rank', $post['serial']) }}" target="_blank">
                          <i class="fas fa-trophy"></i> {{__('home.rank')}}
                        </a>
                      </div>
                    </div>
                    <span class="mt-2 card-text float-left">
                      <button id="popover-button-event{{$post['serial']}}" type="button" class="btn btn-outline-dark btn-sm"
                        @click="share('{{route('game.show',$post['serial'])}}', '{{$post['serial']}}')">
                        {{__('Share')}} &nbsp;<i class="fas fa-share-square"></i>
                      </button>
                      <b-popover ref="popover{{$post['serial']}}" target="popover-button-event{{$post['serial']}}" :disabled="true">
                        {{__('Copied link')}}
                      </b-popover>
                    </span>
                    <span class="mt-2 card-text float-right">
                      <span class="pr-2">
                        <i class="fas fa-play-circle"></i>&nbsp;{{ $post['play_count'] }}
                      </span>
                      <small class="text-muted">{{ $post['created_at'] }}</small>
                    </span>
                  </div>
                </div>
              </div>
            </div>

            
            @if(config('services.google_ad.enabled') && config('services.google_ad.home_page') && $index == count($posts) - 1)
              <div id="google-ad-1" class="grid-item mt-2">
                @include('ads.home_ad_1', ['id' => 'google-ad-1'])
              </div>
            @endif
            @endforeach

            <template v-for="post in posts">
              <home-post
                :show-game-endpoint="showGameEndpoint"
                :show-rank-endpoint="showRankEndpoint"
                :post="post"
                :init-masonry="initMasonry"
                :on-image-error="onImageError"
                @share="handleChildShare"
              >
              </home-post>
            </template>

          </div>

          {{-- post not found  --}}
          @if($posts->count() == 0)
          <div class="text-center">
            <img src="{{ asset('storage/post-not-found-' . app()->getLocale() . '.png') }}" class="img-fluid" alt="{{ __('Not Found') }}">
          </div>
          @endif

          {{-- more posts loading --}}
          <div class="d-flex justify-content-center" style="height: 50px;">
            <div v-if="!isFetchAllPosts && (isLoadingMorePosts || delayLoading)" class="align-content-center">
              <i class="fas fa-spinner fa-spin fa-2x"></i>
            </div>
            <div v-if="isFetchAllPosts" class="align-content-center">
              <span class="text-muted">{{ random_emoji() }}</span>
            </div>
          </div>

          {{-- return to top --}}
          <div v-if="showReturnUpButton" class="align-content-center" style="position: fixed; right: 20px; bottom: 20px;">
            <span class="cursor-pointer" @click="scrollToTop">
              <div class="bg-white text-center align-content-center return-top-button">
                <i class="fas fa-arrow-up"></i>
              </div>
            </span>
          </div>
        </div>
      </div>
    </div>
  </home>

@endsection

@section('footer')
  @include('partial.footer')
@endsection


