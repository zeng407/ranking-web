@php
  $queryParams = request()->query();
  $langPostfixURL = $langPostfixURL ?? '';
  if($queryParams){
    $postFix = $langPostfixURL . '?' . http_build_query(request()->query());
  }else{
    $postFix = $langPostfixURL;
  }

@endphp

<div class="lang-options d-flex flex-column-reverse flex-md-row justify-content-end mt-3 mt-md-0">
    <ul class="list-unstyled">
      <li class="dropdown d-inline-block">
        <a class="nav-link text-dark" href="#" id="dropdownLangButton" role="button" data-toggle="dropdown"
          aria-expanded="false">
          <i class="fas fa-globe-asia"></i>
        </a>
        <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownLangButton">
          <a class="dropdown-item" href="{{'/lang/zh_TW/'. $postFix}}">中文 (Chinese)</a>
          <a class="dropdown-item" href="{{'/lang/en/'. $postFix}}">English</a>
        </div>
      </li>
    </ul>
</div>