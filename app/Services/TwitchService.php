<?php


namespace App\Services;

use Http;
use Cache;
class TwitchService implements InterfaceOauthService
{
    public function refreshAccessToken()
    {
        $client_id = config('services.twitch.client_id');
        $client_secret = config('services.twitch.client_secret');
        $url = "https://id.twitch.tv/oauth2/token?client_id=$client_id&client_secret=$client_secret&grant_type=client_credentials";
        $response = Http::post($url);
        $response = json_decode($response->getBody()->getContents(), true);
        $access_token = $response['access_token'];
        Cache::put('twitch_access_token', $access_token, $response['expires_in']);
    }

    public function getVideo(string $id)
    {
        $client_id = config('services.twitch.client_id');
        $url = "https://api.twitch.tv/helix/videos?id=$id";
        $response = Http::withHeaders([
            'Client-ID' => $client_id,
            'Authorization' => 'Bearer ' . Cache::get('twitch_access_token')
        ])->get($url);
        $response = json_decode($response->getBody()->getContents(), true);
        return $response['data'][0];
    }

    public function getClip($videoId)
    {
        $client_id = config('services.twitch.client_id');
        $url = "https://api.twitch.tv/helix/clips?id=$videoId";
        $response = Http::withHeaders([
            'Client-ID' => $client_id,
            'Authorization' => 'Bearer ' . Cache::get('twitch_access_token')
        ])->get($url);
        $response = json_decode($response->getBody()->getContents(), true);
        return $response['data'][0];
    }

    /**
     * @param $url
     * @return mixed|string
     * @throws \Exception
     */
    public function parseVideoId($url): string
    {
        // try getting video_id format from twitch url 
        // https://www.twitch.tv/videos/xxxxxxxx
        // https://m.twitch.tv/videos/xxxxxxxx
        preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:twitch\.tv))(\/(?:[\w\-]+\?|videos\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
        logger("return matches ");
        logger($matches);
        if($matches && isset($matches[4]) && $matches[4] === '/videos/') {
            return str_replace('/', '', $matches[5] ?? '');
        }
        return '';
    }

    public function parseClipId($url): string
    {
        // try getting clip_slug format from twitch url https://www.twitch.tv/{channel_name}/clip/{clip_slug}
        preg_match('/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:twitch\.tv))(\/(?:[\w\-]+\/clip\/)?)([\w\-]+)(\S+)?$/', $url, $matches);
        logger("return matches ");
        logger($matches);

        if ($matches && isset($matches[5])) {
            return $matches[5];
        }

        return '';
    }


    public function parseChannelId($url): string
    {
        // try getting channel_id format from twitch url https://www.twitch.tv/xxxxx
        preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:twitch\.tv))(\/(?:[\w\-]+\?v=|videos\/|v\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
        logger("return matches ");
        logger($matches);

        if ($matches && isset($matches[6]) && $matches[5] === '/') {
            return str_replace('/', '', $matches[6] ?? '');
        }

        return '';
    }
}
