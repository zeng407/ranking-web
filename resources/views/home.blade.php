@extends('layouts.app', [
    'ogImage' => asset('/storage/og-image.jpeg'),
    'stickyNav' => 'sticky-top',
])

@section('header')
  @if (config('services.google_ad.enabled') && config('services.google_ad.home_page') && !is_skip_ad())
    {{-- Ads --}}
    <script async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js" crossorigin="anonymous"></script>
    <script>
      window.googletag = window.googletag || {
        cmd: []
      };
      googletag.cmd.push(function() {
        var slot = googletag.defineSlot('/23307026516/home_ad_r1', [160, 600], 'div-gpt-ad-1750906757581-0').addService(
          googletag.pubads());
        googletag.pubads().enableSingleRequest();
        googletag.pubads().collapseEmptyDivs();
        googletag.enableServices();
        setInterval(() => {
          googletag.display("div-gpt-ad-1750906757581-0");
          googletag.pubads().refresh([slot]);
        }, 30 * 1000); // 30 seconds
      });

      googletag.cmd.push(function() {
        var slot = googletag.defineSlot('/23307026516/home_ad_r1/home_ad_2', [160, 600], 'div-gpt-ad-1750917306040-0')
          .addService(googletag.pubads());
        googletag.pubads().enableSingleRequest();
        googletag.pubads().collapseEmptyDivs();
        googletag.enableServices();
        setInterval(() => {
          googletag.display("div-gpt-ad-1750917306040-0");
          googletag.pubads().refresh([slot]);
        }, 30 * 1000); // 30 seconds
      });
    </script>
  @endif
@endsection

@section('search')
  {{-- search --}}
  <form class="form-inline my-2 my-lg-0" action="{{ route('home') }}">
    <input id="keyword-input" class="mr-sm-2 search-bar" type="search" placeholder="{{ __('Search') }}"
      value="{{ Request::get('k') }}" name="k" aria-label="Search">
    <button class="btn btn btn-outline-secondary border-0 my-2 my-sm-0 ml-1 ml-sm-0" type="submit"
      @click.prevent="$bus.$emit('initiate-search', $event)">
      <i class="fas fa-search text-white"></i>
    </button>
  </form>
@endsection

@section('content')
  <home inline-template sort-by="{{ $sort ?? 'hot' }}" range="{{ $range ?? config('setting.home_page_default_range') }}"
    keyword="{{ Request::get('k') }}" :current-page={{ $posts->currentPage() }}
    index-posts-endpoint="{{ route('api.public-post.index') }}" index-tags-endpoint="{{ route('api.tag.hot') }}"
    show-game-endpoint="{{ route('game.show', '_serial') }}" show-rank-endpoint="{{ route('game.rank', '_serial') }}"
    get-champions-endpoint="{{ route('api.champion.index') }}">

    <div class="container-fuild">
      {{-- main container --}}
      <div class="row m-0">

        <div class="d-none d-xl-block col-xl-2">
        </div>

        {{-- main part: posts --}}
        <div id="main-region" class="col-12 col-xl-8 p-1 " v-cloak>
          @include('partial.home-carousel')

          {{-- champions --}}
          <h4 class="d-flex my-1 ml-2">@{{ $t('home.new_champions') }}</h4>
          {{-- dummy champion, preserve for champion --}}
          <div v-if="!champions.length" class="d-flex overflow-hidden">
            <div class="row flex-nowrap ml-0">
              {{-- create a empty card --}}
              @for ($i = 0; $i < 3; $i++)
                <div class="card champion-card-container shadow col-auto list-item m-2">
                  <div class="card-body p-2">
                    <div class="row no-gutters">

                      {{-- 左邊佔位區塊 --}}
                      <div class="col-6 px-1">
                        <div class="position-relative">
                          {{-- 使用相同的 class 維持 1:1 比例，但給予灰色背景 --}}
                          <div class="champion-media-box" style="background-color: #e9ecef;"></div>
                        </div>
                        {{-- 標題佔位條 --}}
                        <div class="mt-2 mx-auto"
                          style="height: 14px; width: 80%; background-color: #e9ecef; border-radius: 4px;"></div>
                      </div>

                      {{-- 右邊佔位區塊 --}}
                      <div class="col-6 px-1">
                        <div class="position-relative">
                          <div class="champion-media-box" style="background-color: #e9ecef;"></div>
                        </div>
                        <div class="mt-2 mx-auto"
                          style="height: 14px; width: 80%; background-color: #e9ecef; border-radius: 4px;"></div>
                      </div>
                    </div>

                    {{-- 底部資訊佔位區塊 --}}
                    <div class="row d-flex justify-content-end mt-2 px-2">
                      <div class="w-100 d-flex flex-column align-items-end">
                        <div
                          style="height: 14px; width: 70%; background-color: #e9ecef; border-radius: 4px; margin-bottom: 4px;">
                        </div>
                        <div style="height: 12px; width: 40%; background-color: #e9ecef; border-radius: 4px;"></div>
                      </div>
                    </div>

                  </div>
                </div>
              @endfor
            </div>
          </div>
          <div v-if="champions.length" class="d-flex overflow-x-scroll hide-scrollbar mx-2">
            <transition-group name="list" tag="div" class="row flex-nowrap ml-0">

              <div class="card champion-card-container shadow col-auto list-item m-2" v-for="championResult in champions"
                :key="championResult.key" v-cloak>

                <div class="card-body p-2">
                  <div class="row no-gutters"> {{-- ================= 左邊選手 (Left) ================= --}}
                    <div class="col-6 px-1">
                      <div class="champion-media-box">

                        {{-- 1. 模糊背景 (使用 thumb_url) --}}
                        <div class="champion-media-box__bg"
                          :style="{ backgroundImage: 'url(' + championResult.left.thumb_url + ')' }">
                        </div>

                        {{-- 2. 前景圖片/影片 --}}
                        <video v-if="isEndWith(championResult.left.thumb_url, 'mp4')"
                          @loadeddata="handleCandicateLoaded(championResult.left)"
                          v-show="!isChampionLoading(championResult.left)" class="champion-media-box__content"
                          :class="{ 'eliminated': !championResult.left.is_winner }"
                          :src="championResult.left.thumb_url + '#t=0.01'" muted></video>

                        <img v-else @load="handleCandicateLoaded(championResult.left)"
                          v-show="!isChampionLoading(championResult.left)" class="champion-media-box__content"
                          :class="{ 'eliminated': !championResult.left.is_winner }" :src="championResult.left.thumb_url">


                        {{-- Loading Spinner --}}
                        <div v-show="isChampionLoading(championResult.left)" class="champion-media-box__loading">
                          <i class="fas fa-spinner fa-spin fa-2x text-white"></i>
                        </div>
                      </div>
                      <h5 class="text-center font-size-small mt-2 text-truncate">@{{ championResult.left.name | cut(30) }}</h5>
                    </div>

                    {{-- ================= 右邊選手 (Right) ================= --}}
                    <div class="col-6 px-1" v-if="championResult.right.name">
                      <div class="champion-media-box">

                        {{-- 1. 模糊背景 --}}
                        <div class="champion-media-box__bg"
                          :style="{ backgroundImage: 'url(' + championResult.right.thumb_url + ')' }">
                        </div>

                        {{-- 2. 前景圖片/影片 --}}
                        <video v-if="isEndWith(championResult.right.thumb_url, 'mp4')"
                          @loadeddata="handleCandicateLoaded(championResult.right)"
                          v-show="!isChampionLoading(championResult.right)" class="champion-media-box__content"
                          :class="{ 'eliminated': !championResult.right.is_winner }"
                          :src="championResult.right.thumb_url + '#t=0.01'" muted></video>

                        <img v-else @load="handleCandicateLoaded(championResult.right)"
                          v-show="!isChampionLoading(championResult.right)" class="champion-media-box__content"
                          :class="{ 'eliminated': !championResult.right.is_winner }"
                          :src="championResult.right.thumb_url">

                        {{-- Loading Spinner --}}
                        <div v-show="isChampionLoading(championResult.right)" class="champion-media-box__loading">
                          <i class="fas fa-spinner fa-spin fa-2x text-white"></i>
                        </div>
                      </div>
                      <h5 class="text-center font-size-small mt-2 text-truncate">@{{ championResult.right.name | cut(30) }}</h5>
                    </div>

                  </div>

                  {{-- Footer Info --}}
                  <div class="row d-flex justify-content-end mt-2 px-2">
                    <div class="text-right w-100">
                      <a target="_blank" class="font-size-xsmall d-block text-truncate"
                        :href="getShowGameUrl(championResult.post_serial)">@{{ championResult.post_title }}</a>
                      <p :key="refreshKey" class="d-inline-block font-size-small text-muted mb-0">
                        @{{ humanizeDate(championResult.datetime) }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </transition-group>
          </div>

          <hr id="sorter-hr">
          {{-- sorter --}}
          <div class="d-flex justify-content-between align-items-center flex-nowrap mt-1 ml-2 mr-2">
            <div class="form-inline">
              <div class="btn-group btn-group-toggle" data-toggle="buttons">
                <label class="btn btn-outline-dark white-space-no-wrap">
                  <input type="radio" v-model="filters.sort_by" value="hot">
                  <i class="fas fa-fire-alt"></i>&nbsp;@{{ $t('Hot') }}
                </label>
                <label class="btn btn-outline-dark mr-2 white-space-no-wrap">
                  <input type="radio" v-model="filters.sort_by" value="new">
                  <i class="fas fa-sort-amount-down-alt"></i>&nbsp;@{{ $t('New') }}
                </label>
              </div>
              <div class="form-inline">
                <button type="button" class="btn btn-outline-dark btn-sm rounded-pill" data-toggle="dropdown"
                  v-show="filters.sort_by === 'hot'" style="top: 0">
                  @{{ timeRangeText }}
                  <i class="fas fa-caret-down"></i>
                </button>
                <div class="dropdown-menu">
                  <span class="dropdown-item" @click="clickTimeRange($event, 'month')">@{{ $t('This Month') }}</span>
                  <span class="dropdown-item" @click="clickTimeRange($event, 'week')">@{{ $t('This Week') }}</span>
                  <span class="dropdown-item" @click="clickTimeRange($event, 'day')">@{{ $t('Today') }}</span>
                </div>
              </div>
            </div>
            @include('partial.lang')
          </div>

          {{-- tags --}}
          <div class="tag-container ml-2 mr-2 mt-2 d-flex d-sm-block overflow-x-scroll-sm hide-scrollbar-sm">
            <h5 v-for="tag in tags" class="d-inline-block text-nowrap mr-1 mb-1">
              <span>
                <a class="btn btn-outline-dark btn-sm badge badge-pill badge-secondary" href="#"
                  @click.prevent="addTag(tag.name)">
                  @{{ tag.name }}(@{{ tag.count }})
                </a>
              </span>
            </h5>
          </div>

          {{-- search result --}}
          <div class="d-flex mt-2 mx-2" style=" height:25px">
            <h5 v-if="filters.keyword" class="mr-2 d-inline-block">
              <span class="btn btn-outline-dark badge badge-dark" @click="clearKeyword">
                @{{ $t('Search Results', {
    keyword: filters.keyword
}) }}<i class="fas fa-times ml-1 cursor-pointer"></i>
              </span>
            </h5>
            <div class="d-inline-block position-absolute" style="left: 50%; transform: translateX(-50%);">
              <i v-if="isSearching" class="fas fa-spinner fa-spin fa-2x"></i>
            </div>
          </div>

          {{-- preload posts --}}
          <div class="row m-0">

            @foreach ($posts as $index => $post)
              <div class="col-md-6 col-12 pt-2 preload-post">
                <div class="card shadow">
                  <div class="card-header text-center">
                    <h2 class="post-title">{{ $post['title'] }}</h2>
                  </div>
                  <div class="row no-gutters">
                    <div class="row no-gutters m-0 w-100 position-relative">
                      @if ($post['is_censored'])
                        <image-mask key="image-mask-{{ $post['serial'] }}"></image-mask>
                      @endif

                      {{-- Element 1 (Left) --}}
                      <div class="col-6 px-1 py-2">
                        <div class="comparison-media-box">
                          {{-- 背景模糊層 (僅當是圖片時顯示，若無法取得 thumb 則黑底) --}}
                          @if ($post['element1']['previewable'])
                            <div class="comparison-media-box__bg"
                              style="background-image: url('{{ $post['element1']['url'] }}')"></div>
                          @endif

                          {{-- 內容層 --}}
                          <div class="comparison-media-box__content-wrapper">
                            @if ($post['element1']['previewable'])
                              <flex-image element-id="{{ $post['element1']['id'] }}"
                                thumb-url="{{ $post['element1']['url'] }}" imgur-url="{{ $post['element1']['url2'] }}"
                                alt="{{ $post['element1']['title'] }}" :lazy="true"></flex-image>
                            @else
                              <video src="{{ $post['element1']['url'] }}#t=0.01" muted playsinline></video>
                            @endif
                          </div>
                        </div>
                      </div>

                      {{-- Element 2 (Right) --}}
                      <div class="col-6 px-1 py-2">
                        <div class="comparison-media-box">
                          {{-- 背景模糊層 --}}
                          @if ($post['element2']['previewable'])
                            <div class="comparison-media-box__bg"
                              style="background-image: url('{{ $post['element2']['url'] }}')"></div>
                          @endif

                          {{-- 內容層 --}}
                          <div class="comparison-media-box__content-wrapper">
                            @if ($post['element2']['previewable'])
                              <flex-image element-id="{{ $post['element2']['id'] }}"
                                thumb-url="{{ $post['element2']['url'] }}" imgur-url="{{ $post['element2']['url2'] }}"
                                alt="{{ $post['element2']['title'] }}" :lazy="true"></flex-image>
                            @else
                              <video src="{{ $post['element2']['url'] }}#t=0.01" muted playsinline></video>
                            @endif
                          </div>
                        </div>
                      </div>

                    </div>
                    <div class="row no-gutters m-0 w-100">
                      <div class="col-6">
                        <h5 class="text-center mt-1 p-1 element-title text-truncate">{{ $post['element1']['title'] }}
                        </h5>
                      </div>
                      <div class="col-6">
                        <h5 class="text-center mt-1 p-1 text-truncate">{{ $post['element2']['title'] }}</h5>
                      </div>
                    </div>
                    <div class="card-body pt-0 text-center">
                      <p class="text-break">{{ $post['description'] }}</p>
                      <div class="row">
                        <div class="col-6">
                          <a class="btn btn-primary btn-block" href="{{ route('game.show', $post['serial']) }}"
                            target="_blank">
                            <i class="fas fa-play"></i> {{ __('home.start') }}
                          </a>
                        </div>
                        <div class="col-6">
                          <a class="btn btn-secondary btn-block" href="{{ route('game.rank', $post['serial']) }}"
                            target="_blank">
                            <i class="fas fa-trophy"></i> {{ __('home.rank') }}
                          </a>
                        </div>
                      </div>
                      <span class="mt-2 card-text float-left">
                        <share-link id="{{ $post['serial'] }}" url="{{ route('game.show', $post['serial']) }}"
                          text="{{ __('Share') }}" after-copy-text="{{ __('Copied link') }}"></share-link>
                      </span>
                      <span class="mt-2 card-text float-right white-space-no-wrap">
                        <span class="pr-2">
                          <i class="fas fa-play-circle"></i>&nbsp;{{ $post['play_count'] }}
                        </span>
                        <small class="text-muted white-space-no-wrap">{{ $post['created_at'] }}</small>
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              {{-- ads --}}
              @if (config('services.google_ad.enabled') && config('services.google_ad.home_page') && $index % 12 == 0 && $index > 0)
                <div class="col-md-6 col-12 pt-2 preload-post">
                  @include('ads.home_ad_1_pc')
                </div>
              @endif

              @if (config('services.onead.enabled') &&
                      config('services.onead.home_page') &&
                      (($index + 1) % 6 == 0 || $index == 14) &&
                      $index > 0)
                <div class="col-12 p-4 preload-post">
                  @include('ads.home_onead_2_pc')
                </div>
              @endif
            @endforeach

            <template v-for="(post,index) in posts">
              <home-post :show-game-endpoint="showGameEndpoint" :show-rank-endpoint="showRankEndpoint"
                :post="post" :on-image-error="onImageError" @share="handleChildShare">
              </home-post>

              @if (config('services.google_ad.enabled') && config('services.google_ad.home_page'))
                @if (!is_skip_ad())
                  <div v-if="index % 14 == 0 && index > 0" class="col-md-6 col-12 pt-2 preload-post">
                    <div class="p-4">
                      <div style="width: 100%" class="d-flex justify-content-center">
                        <google-ads pusher-id="{{ config('services.google_ad.publisher_id') }}"
                          slot-id="{{ config('services.google_ad.home_page_ad_1_slot') }}"
                          ins-style="display:inline-block;width:300px;height:300px;" ad-format="rectangle">
                        </google-ads>
                      </div>
                    </div>
                  </div>
                @endif
              @endif

            </template>
          </div>

          {{-- post not found  --}}
          @if ($posts->count() == 0)
            <div class="text-center">
              <img src="{{ asset('storage/post-not-found-' . app()->getLocale() . '.png') }}" class="img-fluid"
                alt="{{ __('Not Found') }}" loading="lazy">
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
          <div v-if="showReturnUpButton" class="align-content-center"
            style="position: fixed; right: 20px; bottom: 20px; z-index:1050;">
            <span class="cursor-pointer" @click="scrollToSorter">
              <div class="bg-secondary text-center align-content-center return-top-button">
                <i class="fas fa-arrow-up text-white"></i>
              </div>
            </span>
          </div>
        </div>

        {{-- right part:ads --}}
        <div class="d-none d-xl-block col-xl-2">
          @if (config('services.google_ad.enabled') && config('services.google_ad.home_page') && !is_skip_ad())
            <div class="pt-2 px-2 mx-auto">
              {{-- google ads --}}
              <!-- /23307026516/home_ad_r1/home_ad_2 -->
              <div id='div-gpt-ad-1750917306040-0' style='min-width: 160px; min-height: 600px;'>
                <script>
                  googletag.cmd.push(function() {
                    googletag.display('div-gpt-ad-1750917306040-0');
                  });
                </script>
              </div>
            </div>

            <div class="pt-2 px-2 mx-auto sticky-top-home-ad">
              {{-- google ads --}}
              <!-- /23307026516/home_ad_r1 -->
              {{-- <div id='div-gpt-ad-1750906757581-0' style='min-width: 160px; min-height: 600px;'>
                <script>
                  googletag.cmd.push(function() {
                    googletag.display('div-gpt-ad-1750906757581-0');
                  });
                </script>
              </div> --}}
              @include('ads.home_ad_1_pc')
            </div>
          @endif
        </div>
      </div>

    </div>
  </home>
@endsection

@section('footer')
  @if (config('services.onead.enabled') && config('services.onead.home_page'))
    @include('ads.home_onead_1_pc')
  @endif
  @include('partial.footer')
@endsection
