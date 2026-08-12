@php
  $queryParams = request()->query();
  $langPostfixURL = $langPostfixURL ?? '';
  $postFix = trim($langPostfixURL, '/');
  $queryString = $queryParams ? '?' . http_build_query($queryParams) : '';

@endphp

<div class="lang-options d-flex flex-column-reverse flex-md-row justify-content-end mt-md-0">
    <ul class="list-unstyled m-0">
      <li class="dropdown d-inline-block">
        <a href="#" class="nav-link text-dark" id="dropdownLangButton" role="button" data-toggle="dropdown"
          aria-expanded="false" @click.prevent>
          <i class="fas fa-globe-asia"></i>
        </a>
        <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownLangButton">
          <a class="dropdown-item" href="{{ localized_path('zh_TW', $postFix) . $queryString }}">{{ __('Chinese') }} (Chinese)</a>
          <a class="dropdown-item" href="{{ localized_path('ja', $postFix) . $queryString }}">{{__('Japanese')}} (Japanese)</a>
          <a class="dropdown-item" href="{{ localized_path('en', $postFix) . $queryString }}">{{__('English')}} (English)</a>
        </div>
      </li>
    </ul>
</div>
