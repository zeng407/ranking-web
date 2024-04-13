@extends('layouts.app', [
  'ogImage' => asset('/storage/og-image.jpeg'),
  'stickyNav' => 'sticky-top'
  ])

@section('content')
    <home
      inline-template
      sort-by="{{$sort ?? 'hot'}}"
      range="{{$range ?? 'month'}}"
      keyword="{{Request::get('k') }}"
    >
    <div class="container-fluid" v-cloak>
      @include('partial.lang')
      <div class="form-inline w-100" >
        <form @submit.prevent class="d-flex w-100 justify-content-center">
          <div class="mr-2 w-75">
            <input class="form-control w-100" v-model="filters.keyword" type="search" placeholder="Search"
              aria-label="Search" >
            <div>
              <div class="row ml-1">
                @foreach($tags as $tag)
                <button type="button" class="btn btn-sm btn-outline-secondary mr-1 mt-1"
                  @click="addTag('{{$tag['name']}}')">
                  #{{ $tag['name'] }}
                </button>
                @endforeach
              </div>
            </div>
          </div>
          <div class="">
            <button class="btn btn-outline-dark w-100" type="submit" @click="search">
              <i class="fas fa-search"></i>
            </button>
          </div>
        </form>
      </div>
  
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
  
      <div class="row justify-content-center pt-4">
        
  
        @foreach($posts as $index => $post)
        <div class="col-xl-4 col-md-6 pt-2">
          <div class="card shadow">
            <div class="card-header text-center">
              <h2>{{ $post['title'] }}</h2>
            </div>
            <div class="row no-gutters">
              <div class="col-6">
                <div class="post-element-container">
                  @if($post['element1']['type'] === \App\Enums\ElementType::IMAGE || $post['element1']['video_source'] === \App\Enums\VideoSource::YOUTUBE || $post['element1']['video_source'] === \App\Enums\VideoSource::YOUTUBE_EMBED)
                  <img src="{{$post['element1']['url']}}" @@error="onImageError('{{$post['element1']['url2']}}', $event)">
                  @else
                  <video src="{{$post['element1']['url']}}#t=1'"></video>
                  @endif
                </div>
                <h5 class="text-center mt-1 p-1">{{ $post['element1']['title'] }}</h5>
              </div>
              <div class="col-6">
                <div class="post-element-container">
                  @if($post['element2']['type'] === \App\Enums\ElementType::IMAGE || $post['element2']['video_source'] === \App\Enums\VideoSource::YOUTUBE || $post['element2']['video_source'] === \App\Enums\VideoSource::YOUTUBE_EMBED)
                  <img src="{{$post['element2']['url']}}" @@error="onImageError('{{$post['element2']['url2']}}', $event)">
                  @else
                  <video src="{{$post['element2']['url']}}#t=1'"></video>
                  @endif
                </div>
                <h5 class="text-center mt-1 p-1">{{ $post['element2']['title'] }}</h5>
              </div>
              <div class="card-body pt-0 text-center">
                <p class="text-break">{{ $post['description'] }}</p>
                <div class="row">
                  <div class="col-6">
                    <a class="btn btn-primary btn-block" href="{{route('game.show', $post['serial'])}}">
                      <i class="fas fa-play"></i> {{__('home.start')}}
                    </a>
                  </div>
                  <div class="col-6">
                    <a class="btn btn-secondary btn-block" href="{{route('game.rank', $post['serial']) }}">
                      <i class="fas fa-trophy"></i> {{__('home.rank')}}
                    </a>
                  </div>
                </div>
                <span class="mt-2 card-text float-left">
                  <button id="popover-button-event{{$index}}" type="button" class="btn btn-outline-dark btn-sm"
                    @click="share('{{route('game.show',$post['serial'])}}',{{$index}})">
                    {{__('Share')}} &nbsp;<i class="fas fa-share-square"></i>
                  </button>
                  <b-popover ref="popover{{$index}}" target="popover-button-event{{$index}}" :disabled="true">
                    {{__('Copied')}}</strong>
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
        @endforeach
      </div>
  
      <div class="row justify-content-center pt-2">
        {{ $posts->appends(request()->except('page'))->links() }}
      </div>
  
    </div>
  </home>

@endsection

@section('footer')
  @include('partial.footer')
@endsection


