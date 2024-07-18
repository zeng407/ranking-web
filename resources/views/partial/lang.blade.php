@php
  $queryParams = request()->query();
  $langPostfixURL = $langPostfixURL ?? '';
  if($queryParams){
    $postFix = $langPostfixURL . '?' . http_build_query(request()->query());
  }else{
    $postFix = $langPostfixURL;
  }

@endphp

<div class="lang-options d-flex flex-column-reverse flex-md-row justify-content-end mt-md-0">
    <ul class="list-unstyled m-0">
      <li class="dropdown d-inline-block">
        <a href="#" class="nav-link text-dark" id="dropdownLangButton" role="button" data-toggle="dropdown"
          aria-expanded="false" @click.prevent>
          <i class="fas fa-globe-asia"></i>
        </a>
        <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownLangButton">
          <a class="dropdown-item" href="{{'/lang/zh_TW/'. $postFix}}">{{ __('Chinese') }} (Chinese)</a>
          <a class="dropdown-item" href="{{'/lang/ja/'. $postFix}}">{{__('Japanese')}} (Japanese)</a>
          <a class="dropdown-item" href="{{'/lang/en/'. $postFix}}">{{__('English')}} (English)</a>
        </div>
      </li>
    </ul>
</div>
