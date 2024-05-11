<?php

namespace App\Services\ElementHandlers;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use App\Enums\ElementType;

class YoutubeEmbedElementHandler implements InterfaceElementHandler
{
    use FileHelper;

    public function storeArray(string $embedCode, string $serial, $params = []): ?array
    {
        // extract video id from embed code
        preg_match('/src="https:\/\/www.youtube.com\/embed\/([^"]+)"/', $embedCode, $matches);
        $videoUrl = $matches[1] ?? null;
        
        // validate video id 
        // example : 1H2cyhWYXrE?si=btfjgIQDNUoNuriT&amp;clip=UgkxeWL6j9ODyTnJpJe6Ris_NgNzLFls3SyG&amp;clipt=ELidBRjQkgY
        $validate = preg_match('/^[a-zA-Z0-9?&;=_-]+$/', $videoUrl) && strlen($videoUrl) <= 120;
        if (!$validate){
            return null;
        }
        $videoParams = explode('?', $videoUrl);
        $videoId = $videoParams[0];
        if(isset($videoParams[1])){
            $embedUrl = $videoId.'?' . $videoParams[1].'&autoplay=1&playlist='.$videoId.'&loop=1';
        }else{
            $embedUrl = $videoId.'?autoplay=1&playlist='.$videoId.'&loop=1';
        }
        $embedCode = "<iframe width=\"100%\" height=\"270\" src=\"https://www.youtube.com/embed/{$embedUrl}\" title=\"YouTube video player\" " . 
            "frameborder=\"0\" allow=\"accelerometer; loop; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share\" ". 
            "referrerpolicy=\"strict-origin-when-cross-origin\" allowfullscreen ></iframe>";
        $thumbUrl = "https://img.youtube.com/vi/{$videoId}/hqdefault.jpg";
        
        return [
            'title' => $params['title'] ?? '',
            'thumb_url' => $thumbUrl,
            'embed_code' => $embedCode,
            'video_id' => $videoId,
        ];
    }

    public function storeElement(string $embedCode, Post $post, $params = []): ?Element
    {
        $array = $this->storeArray($embedCode, $post->serial, $params);
        if(!$array){
            return null;
        }

        $element = $post->elements()->updateOrCreate([
            'source_url' => $params['old_source_url'] ??  '',
        ], [
            'source_url' => $embedCode,
            'thumb_url' => $array['thumb_url'],
            'type' => ElementType::VIDEO,
            'title' => $array['title'],
            'video_source' => VideoSource::YOUTUBE_EMBED,
            'video_id' => $array['video_id'],
        ]);

        return $element;
    }
}