@if ($rank->loser->type === 'video' && $rank->loser->video_source === 'youtube')
    <youtube-player width="100%"  ref-id="{{ $rank->loser->id }}" video-id="{{ $rank->loser->video_id }}"
        :controls="true" :autoplay="false" :rel="0"
        origin="{{ request()->getSchemeAndHttpHost() }}">
    @elseif($rank->loser->type === 'video' && $rank->loser->video_source === 'youtube_embed')
        {!! inject_youtube_embed($rank->loser->source_url, ['autoplay' => false]) !!}
    @elseif($rank->loser->type === 'video' && $rank->loser->video_source === 'bilibili_video')
        <bilibili-video width="100%"
            :element="{{ json_encode(['id' => $rank->loser->id, 'video_id' => $rank->loser->video_id, 'thumb_url' => $rank->loser->thumb_url]) }}"
            :autoplay="false" :muted="false" :preview="true">
        </bilibili-video>
    @elseif($rank->loser->type === 'video' && $rank->loser->video_source === 'twitch_video')
        <iframe
            src="https://player.twitch.tv/?video={{ $rank->loser->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
             width="100%" allowfullscreen>
        </iframe>
    @elseif($rank->loser->type === 'video' && $rank->loser->video_source === 'twitch_clip')
        <iframe
            src="https://clips.twitch.tv/embed?clip={{ $rank->loser->video_id }}&parent={{ request()->getHost() }}&autoplay=false"
             width="100%" allowfullscreen>
        </iframe>
    @elseif($rank->loser->type === 'video')
        <video width="100%"  loop controls playsinline src="{{ $rank->loser->source_url }}" poster="{{ $rank->loser->thumb_url }}"></video>
    @elseif($rank->loser->type === 'image')
        <viewer :options="viewerOptions">
            <img class="w-auto mw-100 cursor-pointer"
              src="{{ $rank->loser->lowthumb_url ?: $rank->loser->thumb_url }}"
              srcset="{{ $rank->loser->lowthumb_url ?: $rank->loser->thumb_url }} 400w,
                  {{ $rank->loser->mediumthumb_url ?: $rank->loser->thumb_url }} 800w"
              sizes="(max-width: 400px) 400px, 800px"
              alt="{{ $rank->loser->title }}">
        </viewer>
@endif
