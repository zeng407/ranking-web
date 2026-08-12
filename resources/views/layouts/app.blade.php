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

  {{-- Google ad --}}
  @if (config('services.google_ad.enabled'))
    @if (!is_skip_ad())
      <script async
        src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
        crossorigin="anonymous"></script>
      <script async src="https://fundingchoicesmessages.google.com/i/pub-3442386930660042?ers=1"></script><script>(function() {function signalGooglefcPresent() {if (!window.frames['googlefcPresent']) {if (document.body) {const iframe = document.createElement('iframe'); iframe.style = 'width: 0; height: 0; border: none; z-index: -1000; left: -1000px; top: -1000px;'; iframe.style.display = 'none'; iframe.name = 'googlefcPresent'; document.body.appendChild(iframe);} else {setTimeout(signalGooglefcPresent, 0);}}}signalGooglefcPresent();})();</script>
    @endif
  @endif

  {{-- gad --}}
  @if(config('services.gad.enabled'))
  <script src="https://storage.googleapis.com/teleworker-public-storage/gad_js/net.min.js" async></script>
  @endif

  <script>
    // prevent form resubmission on page refresh
    document.addEventListener("DOMContentLoaded", function() {
      if (window.history.replaceState) {
          var url = window.location.href;

          // replaceState(state, title, url)
          window.history.replaceState(null, null, url);
      }
    });
  </script>

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
  @if (!request()->attributes->get(\App\Http\Middleware\PublicHtmlCacheHeaders::ENABLED_ATTRIBUTE, false))
    <meta name="csrf-token" content="{{ csrf_token() }}">
  @endif

  {{-- Scripts --}}
  <script src="{{ mix('js/app.js') }}" defer></script>

  {{-- Fonts --}}
  <link rel="dns-prefetch" href="//fonts.gstatic.com">
  <link href="https://fonts.googleapis.com/css?family=Nunito" rel="stylesheet">

  {{-- Styles --}}
  <link href="{{ mix('css/app.css') }}" rel="stylesheet">
  <link href="{{ app()->isProduction() ? mix('css/prod.css') : mix('css/local.css') }}" rel="stylesheet">
</head>

<body class="d-flex flex-column min-vh-100 @if (isset($darkTheme) && $darkTheme) game-dark-theme @endif">
  <div id="app">
    @if (!isset($embed) || !$embed)
      <nav class="navbar navbar-expand-md navbar-dark bg-dark shadow-sm {{ isset($stickyNav) ? $stickyNav : '' }}">
        <div class="container-fluid">
          <div class="d-flex justify-content-start">
            {{-- logo --}}
            <a class="navbar-brand" href="{{ locale_aware_path('/') }}" title="{{ config('app.short_name') }}">
              <img src="{{ asset('storage/logo.png') }}" class="d-inline-block align-top home-logo"
                alt="{{ config('app.short_name') }}">
            </a>
            {{-- posts --}}
            <ul class="navbar-nav mr-auto">
              <li class="nav-item d-none d-md-block">
                <h1 class="font-size-large m-0">
                  <a class="nav-link" href="{{ locale_aware_path('/') }}"
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
            <navbar-auth
              context-endpoint="{{ route('session.context', request()->route('locale') ? ['locale' => app()->getLocale()] : []) }}"
              login-url="{{ locale_aware_path('/login') }}"
              posts-url="{{ route('post.index') }}"
              profile-url="{{ route('profile.index') }}"
              logout-url="{{ route('logout') }}"
              donate-url="{{ locale_aware_path('/donate') }}"
              default-avatar-url="{{ asset('storage/default-avatar.webp') }}"
              login-label="{{ __('Login & New Post') }}"
              posts-label="{{ __('My Votes') }}"
              profile-label="{{ __('Profile') }}"
              logout-label="{{ __('Logout') }}"
              donate-label="{{ __('Donate') }}"
              avatar-alt="{{ __('Avatar') }}"
              :show-donate="{{ app()->getLocale() == 'zh_TW' ? 'true' : 'false' }}">
            </navbar-auth>
          </div>

        </div>
      </nav>
    @endif

    <main>
      @include('layouts.flash')
      @yield('content')
      <announcement :announcement="{{ json_encode(\App\Helper\CacheService::rememberAnnouncement()) }}"></announcement>

      {{-- @if (!is_skip_ad())
        @include('ads.remove-ad')
      @endif --}}

    </main>
  </div>
  @yield('footer')
  @stack('scripts')
</body>

</html>
