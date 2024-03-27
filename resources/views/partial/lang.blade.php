<div class="d-flex justify-content-end">
    <ul class="list-unstyled">
      <li class="dropdown">
        <a class="nav-link text-dark" href="#" id="dropdownLangButton" role="button" data-toggle="dropdown"
          aria-expanded="false">
          <i class="fas fa-globe-asia"></i>
        </a>
        <div class="dropdown-menu dropdown-menu-right" aria-labelledby="dropdownLangButton">
          <a class="dropdown-item" href="{{'/lang/zh_TW/'. ($langPostfixURL ?? '')}}">中文 (Chinese)</a>
          <a class="dropdown-item" href="{{'/lang/en/'. ($langPostfixURL ?? '')}}">English</a>
        </div>
      </li>
    </ul>
</div>