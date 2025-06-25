<!doctype html>
<html lang="{{ str_replace('_', '-', app()->getLocale()) }}">

<head>
  @if (config('services.google_analytics.id'))
    {{-- Google tag (gtag.js) --}}
    <script script async src="https://www.googletagmanager.com/gtag/js?id={{ config('services.google_analytics.id') }}">
    </script>
    <script>
      window.dataLayer = window.dataLayer || [];

      function gtag() {
        dataLayer.push(arguments);
      }
      gtag('js', new Date());

      gtag('config', '{{ config('services.google_analytics.id') }}');
    </script>
  @endif

  @if (config('services.ezoic.enabled'))
    <script src="https://cmp.gatekeeperconsent.com/min.js" data-cfasync="false"></script>
    <script src="https://the.gatekeeperconsent.com/cmp.min.js" data-cfasync="false"></script>
    <script async src="//www.ezojs.com/ezoic/sa.min.js"></script>
    <script>
      window.ezstandalone = window.ezstandalone || {};
      ezstandalone.cmd = ezstandalone.cmd || [];
    </script>
  @endif

  {{-- Google ad --}}
  @if (config('services.google_ad.enabled'))
    @if (!is_skip_ad())
      <script async
        src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
        crossorigin="anonymous"></script>
    @endif
  @endif

  {{-- SEO --}}
  <title>{{ get_page_title($title ?? '') }}</title>
  <script type="application/ld+json">
        {
          "@context" : "https://schema.org",
          "@type" : "WebSite",
          "alternateName": ["{!! str_replace(',' ,'","',config('app.short_names'))!!}"],
          "name" : "{{config('app.name')}}",
          "url" : "{{config('app.url')}}"
        }
    </script>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="description" content="{{ get_page_description($post ?? null) }}">
  <meta property="og:site_name" content="{{ get_page_title($ogTitle ?? null) }}" />
  <meta property="og:title" content="{{ get_page_title($ogTitle ?? null) }}" />
  <meta property="og:image" content="{{ $ogImage ?? asset('/storage/og-image.jpeg') }}" />
  <meta property="og:description" content="{{ $ogDescription ?? get_page_description($post ?? null) }}" />
  <link rel="icon" href="/favicon.ico">

  @yield('header')
  {{-- CSRF Token --}}
  <meta name="csrf-token" content="{{ csrf_token() }}">

  {{-- Scripts --}}
  <script src="{{ mix('js/app.js') }}" defer></script>

  {{-- Fonts --}}
  <link rel="dns-prefetch" href="//fonts.gstatic.com">
  <link href="https://fonts.googleapis.com/css?family=Nunito" rel="stylesheet">

  {{-- Styles --}}
  <link href="{{ mix('css/app.css') }}" rel="stylesheet">
  <link href="{{ app()->isProduction() ? mix('css/prod.css') : mix('css/local.css') }}" rel="stylesheet">
</head>

<body class="d-flex flex-column min-vh-100">
  <div id="app">
    @if (!isset($embed) || !$embed)
      <nav class="navbar navbar-expand-md navbar-dark bg-dark shadow-sm {{ isset($stickyNav) ? $stickyNav : '' }}">
        <div class="container-fluid">
          <div class="d-flex justify-content-start">
            {{-- logo --}}
            <a class="navbar-brand" href="{{ route('home') }}" title="{{ config('app.short_name') }}">
              <img src="{{ asset('storage/logo.png') }}" class="d-inline-block align-top home-logo"
                alt="{{ config('app.short_name') }}">
            </a>
            {{-- posts --}}
            <ul class="navbar-nav mr-auto">
              <li class="nav-item d-none d-md-block">
                <h1 class="font-size-large m-0">
                  <a class="nav-link" href="{{ route('home') }}"
                    title="{{ __('home.posts') }}">{{ __('home.posts') }}</a>
                </h1>
              </li>
            </ul>
          </div>

          <div class="d-flex justify-content-center">
            @yield('search')
          </div>

          {{-- menu --}}
          <div class="d-flex d-md-none justify-content-end">
            <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarSupportedContent"
              aria-controls="navbarSupportedContent" aria-expanded="false" aria-label="{{ __('Toggle navigation') }}">
              <span class="navbar-toggler-icon"></span>
            </button>
          </div>

          {{-- profile --}}
          <div class="collapse navbar-collapse text-right" id="navbarSupportedContent" style="flex-grow:inherit">
            {{-- Left Side Of Navbar --}}
            <ul class="navbar-nav mr-auto">

            </ul>

            {{-- Right Side Of Navbar --}}
            <ul class="navbar-nav ml-auto">
              @if (app()->getLocale() == 'zh_TW')
                <li class="nav-item">
                  <a class="nav-link" href="{{ route('donate') }}">{{ __('Donate') }}</a>
                </li>
              @endif
              {{-- Authentication Links --}}
              @guest
                <li class="nav-item">
                  <a class="nav-link" href="{{ route('login') }}">{{ __('Login & New Post') }}</a>
                </li>
              @else
                <li class="nav-item">
                  <a class="nav-link" href="{{ route('post.index') }}">{{ __('My Votes') }}</a>
                </li>
                <li class="nav-item dropdown">
                  <a id="navbarDropdown" class="nav-link dropdown-toggle" href="#" role="button"
                    data-toggle="dropdown" aria-haspopup="true" aria-expanded="false" v-pre>
                    <img src="{{ auth()->user()->avatar_url ?? asset('storage/default-avatar.webp') }}"
                      class="rounded-circle" width="24" height="24" style="object-fit: cover"
                      alt="{{ __('Avatar') }}">
                    <span class="caret"></span>
                  </a>
                  <div class="dropdown-menu dropdown-menu-right" aria-labelledby="navbarDropdown">
                    <a class="dropdown-item" href="{{ route('profile.index') }}">
                      {{ __('Profile') }}
                    </a>
                    <a class="dropdown-item" href="{{ route('logout') }}"
                      onclick="event.preventDefault();document.getElementById('logout-form').submit();">
                      {{ __('Logout') }}
                    </a>

                    <form id="logout-form" action="{{ route('logout') }}" method="POST" class="d-none">
                      @csrf
                    </form>
                  </div>
                </li>
              @endguest
            </ul>
          </div>

        </div>
      </nav>
    @endif

    <main>
      @include('layouts.flash')
      @yield('content')
      <announcement :announcement="{{ json_encode(\App\Helper\CacheService::rememberAnnouncement()) }}"></announcement>

      @if (!is_skip_ad())
        @include('ads.remove-ad')
      @endif

    </main>
  </div>
  @yield('footer')
</body>

</html>
