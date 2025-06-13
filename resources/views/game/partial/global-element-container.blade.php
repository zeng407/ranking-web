@if ($rank->element->type === 'video' && $rank->element->video_source === 'youtube')
    <youtube-player width="100%" ref-id="{{ $rank->element->id }}"
        video-id="{{ $rank->element->video_id }}" :controls="true" :autoplay="false" :rel="0"
        origin="{{ request()->getSchemeAndHttpHost() }}">
    </youtube-player>
@elseif ($rank->element->type === 'video' && $rank->element->video_source === 'youtube_embed')
    {!! inject_youtube_embed($rank->element->source_url, ['autoplay' => false]) !!}
@elseif($rank->element->type === 'video' && $rank->element->video_source === 'twitch_video')
    <iframe
        src="https://player.twitch.tv/?video={{ $rank->element->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
        width="100%" allowfullscreen>
    </iframe>
@elseif($rank->element->type === 'video' && $rank->element->video_source === 'twitch_clip')
    <iframe
        src="https://clips.twitch.tv/embed?clip={{ $rank->element->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
        width="100%" allowfullscreen>
    </iframe>
@elseif ($rank->element->type === 'video' && $rank->element->video_source === 'bilibili_video')
    <bilibili-video width="100%"
        :element="{{ json_encode(['id' => $rank->element->id, 'video_id' => $rank->element->video_id, 'thumb_url' => $rank->element->thumb_url]) }}"
        :autoplay="false" :muted="false" :preview="true">
    </bilibili-video>
@elseif($rank->element->type === 'video')
    <video width="100%" loop controls playsinline src="{{ $rank->element->source_url }}" poster="{{ $rank->element->thumb_url }}"></video>
@elseif($rank->element->type === 'image')
    <viewer :options="viewerOptions">
      <flex-image
        class="mw-100 cursor-pointer"
        key="{{$rank->element->id}}"
        image-key="{{$rank->element->id}}"
        element-id="{{$rank->element->id}}"
        imgur-url="{{$rank->element->getImgurUrl()}}"
        thumb-url="{{$rank->element->getDefaultThumbUrl()}}"
        thumb-url2="{{$rank->element->getMediumThumbUrl()}}"
        alt="{{$rank->element->title}}"
        ></flex-image>
    </viewer>
@endif
