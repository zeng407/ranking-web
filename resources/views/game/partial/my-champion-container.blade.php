@if ($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'youtube')
    <youtube-player width="100%" height="270" ref-id="{{ $gameResult->winner->id }}"
        video-id="{{ $gameResult->winner->video_id }}" :controls="true" :autoplay="false"
        :rel="0" origin="{{ request()->getSchemeAndHttpHost() }}">
    </youtube-player>
@elseif($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'youtube_embed')
    {!! inject_youtube_embed($gameResult->winner->source_url, ['autoplay' => false]) !!}
@elseif($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'twitch_video')
    <iframe src="https://player.twitch.tv/?video={{ $gameResult->winner->video_id }}&parent={{ request()->getHost() }}"
        height="270" width="100%" allowfullscreen>
    </iframe>
@elseif($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'twitch_clip')
    <iframe
        src="https://clips.twitch.tv/embed?clip={{ $gameResult->winner->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
        height="270" width="100%" allowfullscreen>
    </iframe>
@elseif($gameResult->winner->type === 'video' && $gameResult->winner->video_source === 'bilibili_video')
    <bilibili-video width="100%" height="270"
        :element="{{ json_encode(['id' => $gameResult->winner->id, 'video_id' => $gameResult->winner->video_id, 'thumb_url' => $gameResult->winner->thumb_url]) }}"
        :autoplay="false" :muted="false" height="270" :preview="true">
    </bilibili-video>
@elseif($gameResult->winner->type === 'video')
    <video width="100%" height="270" loop controls playsinline src="{{ $gameResult->winner->source_url }}" poster="{{ $gameResult->winner->thumb_url }}"></video>
@elseif($gameResult->winner->type === 'image')
    <viewer image="{{ $gameResult->winner->thumb_url }}" :options="viewerOptions">
        <img class="w-auto mw-100 cursor-pointer my-champion-element" src="{{ $gameResult->winner->thumb_url }}"
            alt="{{ $gameResult->winner->title }}">
    </viewer>
@endif
