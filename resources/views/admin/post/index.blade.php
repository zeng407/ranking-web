@extends('admin.layouts.app', ['title' => __('管理後台 - 貼文管理')])

@section('content')
    <div class="container">
        <h1 class="mt-3">{{ __('貼文管理') }}</h1>
        <table class="table table-bordered w-100">
            <!-- Table headers -->
            <thead>
                <tr>
                    <th>#</th>
                    <th class="w-25 text-nowrap">{{ __('標題') }}</th>
                    <th class="w-25">{{ __('描述') }}</th>
                    <th class="text-nowrap">{{ __('素材數') }}</th>
                    <th class="text-nowrap">{{ __('發佈') }}</th>
                    <th class="text-nowrap">{{ __('會員') }}</th>
                    <th class="text-nowrap">{{ __('上傳日') }}</th>
                    <th class="text-nowrap">{{ __('查看') }}</th>
                </tr>
            </thead>
            <!-- Table body -->
            <tbody>
                @foreach ($posts as $post)
                    <tr>
                        <td>{{$post->id}}</td>
                        <td class="text-break">{{ $post->title }}</td>
                        <td class="text-break">{{ $post->description }}</td>
                        <td>{{ $post->elements()->count() }}</td>
                        <td>{{ $post->post_policy->getAccessPolicyEnum() }}</td>
                        <td >{{ $post->user->name }}<br>
                            <span class="badge badge-secondary text-truncate" style="max-width: 150px;">{{ $post->user->email }}aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</span>
                            
                        </td>
                        <td>{{ $post->created_at->diffForHumans() }}<br>
                            <span class="badge badge-secondary">{{ $post->created_at->format('Y-m-d H:i') }}</span>
                        </td>
                        <td><a href="{{route('admin.post.show', $post->id)}}"><i class="fas fa-search"></i></a></td>
                    </tr>
                @endforeach
            </tbody>
        </table>
        <!-- Pagination links -->
        {{ $posts->links() }}
    </div>
@endsection
