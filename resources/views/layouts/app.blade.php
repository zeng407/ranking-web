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
        <script async
            src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client={{ config('services.google_ad.publisher_id') }}"
            crossorigin="anonymous"></script>
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
            <nav
                class="navbar navbar-expand-md navbar-dark bg-dark shadow-sm {{ isset($stickyNav) && $stickyNav ? 'sticky-top' : '' }}">
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
                                <a class="nav-link" href="{{ route('home') }}"
                                    title="{{ __('home.posts') }}">{{ __('home.posts') }}</a>
                            </li>
                        </ul>
                    </div>

                    <div class="d-flex justify-content-center">
                        {{-- search --}}
                        <form class="form-inline my-2 my-lg-0" action="{{ route('home') }}" >
                            <input class="mr-sm-2 search-bar" type="search" placeholder="{{ __('Search') }}"
                                value="{{ Request::get('k') }}" name="k"
                                aria-label="Search">
                            <button class="btn btn-outline-white my-2 my-sm-0" type="submit">
                                <i class="fas fa-search text-white"></i>
                            </button>
                        </form>
                    </div>

                    {{-- menu --}}
                    <div class="d-flex d-md-none justify-content-end">
                        <button class="navbar-toggler" type="button" data-toggle="collapse"
                            data-target="#navbarSupportedContent" aria-controls="navbarSupportedContent"
                            aria-expanded="false" aria-label="{{ __('Toggle navigation') }}">
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
                                            class="rounded-circle" width="30" height="30" style="object-fit: cover"
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

                                        <form id="logout-form" action="{{ route('logout') }}" method="POST"
                                            class="d-none">
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

        <main class="mt-2">
            @include('layouts.flash')
            @yield('content')
        </main>
    </div>

    @yield('footer')
</body>

</html>
