<div class="row p-2">
    <a class="btn btn-lg btn-outline-light m-1 ml-1" href="{{ route('admin.post.index') }}">
        {{ __('貼文管理') }}
    </a>
    <div class="dropdown">
        <button class="btn btn-lg btn-outline-light dropdown-toggle m-1" type="button" id="postManagementDropdown"
            data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
            {{ __('會員管理') }}
        </button>
        <div class="dropdown-menu" aria-labelledby="postManagementDropdown">
            <a class="dropdown-item" href="{{ route('admin.user.search') }}">會員查詢</a>
            <a class="dropdown-item" href="{{ route('admin.user.index') }}">會員列表</a>
        </div>
    </div>
    <a class="btn btn-lg btn-outline-light m-1" href="{{ route('admin.home-carousel') }}">
        {{ __('首頁輪播') }}
    </a>
    <a class="btn btn-lg btn-outline-light m-1" href="{{ route('admin.announcement.index') }}">
      {{ __('公告') }}
  </a>
</div>
