<?php


namespace App\Services;

class TwitchService
{
    private array $cache = [];

    /**
     * @param $url
     * @return mixed|string
     * @throws \Exception
     */
    public function parseVideoId($url): string
    {
        // try getting video_id format from twitch url https://www.twitch.tv/videos/xxxxx
        preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:twitch\.tv))(\/(?:[\w\-]+\?v=|videos\/|v\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
        logger("return matches ");
        logger($matches);

        if ($matches && isset($matches[6]) && $matches[5] === '/videos/') {
            return str_replace('/', '', $matches[6] ?? '');
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
