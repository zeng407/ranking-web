<?php


namespace App\Services;
use App\Enums\VideoSource;
use App\Models\HomeCarouselItem;
use App\Services\ElementHandlers\TwitchElementHandler;
use App\Services\ElementHandlers\YoutubeElementHandler;

class HomeCarouselService
{
    public function getHomeCarouselItems()
    {
        return HomeCarouselItem::where('is_active',true)
            ->orderBy('position')->get();
    }

    public function createHomeCarouselItem($data)
    {
        if($data['type'] === 'video') {
            return $this->handleCreateVideo($data);   
        }

        return null;
    }

    
    protected function handleCreateVideo($data):?HomeCarouselItem
    {
        $guess = new ElementSourceGuess();
        $guess->guess($data['video_url']);
        $params = [
            'title' => $data['title'] ?? null,
            'video_start_second' => $data['video_start_second'] ?? null,
            'is_active' => $data['is_active'] ?? true,
        ];
        
        switch (true){
            case $guess->isYoutube:
                return $this->handleYoutubeVideo(
                    (new YoutubeElementHandler())->storeArray($data['video_url'], $serial = 'carousel', $params));
            case $guess->isTwitch:
                logger('twitch');
                return $this->handleTwitchVideo(
                    (new TwitchElementHandler())->storeArray($data['video_url'], $serial = 'carousel', $params));
        }
    }

    protected function handleYoutubeVideo(?array $data): ?HomeCarouselItem
    {
        if(!$data) {
            return null;
        }

        return HomeCarouselItem::create([
            'title' => $data['title'],
            'description' => $data['title'],
            'image_url' => $data['thumb_url'],
            'video_url' => $data['source_url'],
            'video_id' => $data['video_id'],
            'position' => 1,
            'type' => 'video',
            'video_source' => VideoSource::YOUTUBE,
            'video_start_second' => $data['video_start_second'] ?? null,
            'is_active' => $data['is_active'] ?? true,
        ]);
    }

    protected function handleTwitchVideo(?array $data):?HomeCarouselItem
    {
        logger($data);
        if(!$data) {
            return null;
        }

        return HomeCarouselItem::create([
            'title' => $data['title'],
            'description' => $data['title'],
            'image_url' => $data['thumb_url'],
            'video_url' => $data['source_url'],
            'video_id' => $data['video_id'],
            'video_source' => $data['video_source'],
            'video_start_second' => $data['video_start_second'],
            'position' => 1,
            'type' => 'video',
            'is_active' => $data['is_active'] ?? true,
        ]);
    }

}
