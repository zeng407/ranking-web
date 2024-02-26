<div class="row">
    <a class="btn btn-lg btn-outline-light m-1" href="{{route('admin.post.index')}}">
        {{ __('貼文管理') }}
    </a>
    <div class="dropdown">
        <button class="btn btn-lg btn-outline-light dropdown-toggle m-1" type="button" id="postManagementDropdown"
            data-toggle="dropdown" aria-haspopup="true" aria-expanded="false">
            {{ __('帳號管理') }}
            <div class="dropdown-menu" aria-labelledby="postManagementDropdown">
                <a class="dropdown-item" href="">Option 1</a>
                <a class="dropdown-item" href="#">Option 2</a>
                <a class="dropdown-item" href="#">Option 3</a>
            </div>
        </button>
    </div>
</div>
