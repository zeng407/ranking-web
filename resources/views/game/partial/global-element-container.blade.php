@php
if(!isset($height)) $height = 270
@endphp

@if ($rank->element->type === 'video' && $rank->element->video_source === 'youtube')
    <youtube-player width="100%" height="{{$height}}" ref-id="{{ $rank->element->id }}"
        video-id="{{ $rank->element->video_id }}" :controls="true" :autoplay="false" :rel="0"
        origin="{{ request()->getSchemeAndHttpHost() }}">
    </youtube-player>
@elseif ($rank->element->type === 'video' && $rank->element->video_source === 'youtube_embed')
    {!! inject_youtube_embed($rank->element->source_url, ['autoplay' => false]) !!}
@elseif($rank->element->type === 'video' && $rank->element->video_source === 'twitch_video')
    <iframe
        src="https://player.twitch.tv/?video={{ $rank->element->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
        height="{{$height}}" width="100%" allowfullscreen>
    </iframe>
@elseif($rank->element->type === 'video' && $rank->element->video_source === 'twitch_clip')
    <iframe
        src="https://clips.twitch.tv/embed?clip={{ $rank->element->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
        height="{{$height}}" width="100%" allowfullscreen>
    </iframe>
@elseif ($rank->element->type === 'video' && $rank->element->video_source === 'bilibili_video')
    <bilibili-video width="100%" height="{{$height}}"
        :element="{{ json_encode(['id' => $rank->element->id, 'video_id' => $rank->element->video_id, 'thumb_url' => $rank->element->thumb_url]) }}"
        :autoplay="false" :muted="false" height="{{$height}}" :preview="true">
    </bilibili-video>
@elseif($rank->element->type === 'video')
    <video width="100%" height="{{$height}}" loop controls playsinline src="{{ $rank->element->source_url }}" poster="{{ $rank->element->thumb_url }}"></video>
@elseif($rank->element->type === 'image')
    <viewer :options="viewerOptions">
        <img class="w-auto mw-100 cursor-pointer"
          src="{{ $rank->element->getScaledThumbUrl()}}"
          height="{{$height}}"
          alt="{{ $rank->element->title }}">
    </viewer>
@endif
