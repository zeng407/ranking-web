<?php


namespace App\Services;

use Google\Client;
use Google\Service\YouTube;

class YoutubeService
{
    const YOUTUBE_VIDEO_ID_LENGTH = 11;
    protected $youtube;

    public function __construct()
    {
        $client = new Client();
        $client->setApplicationName(config('app.name'));
        $client->setDeveloperKey(config('google.YOUTUBE_API_KEY'));
        $this->youtube = new YouTube($client);
    }

    /**
     * @param $url
     * @return mixed|string
     * @throws \Exception
     */
    public function parseVideoId($url): string
    {
        $errors = [];
        try {
            //try getting v={video_id} format
            parse_str(parse_url($url, PHP_URL_QUERY), $result);
            return $result['v'];
        } catch (\Throwable $throwable) {
            $errors[] = $throwable->getMessage();
        }

        try {
            //try getting {video_id} format
            if (strlen($url) === self::YOUTUBE_VIDEO_ID_LENGTH) {
                return $url;
            }
        } catch (\Throwable $throwable) {

        }

        try {
            //try getting url?v={video_id} format
            preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:youtube(-nocookie)?\.com|youtu.be))(\/(?:[\w\-]+\?v=|embed\/|v\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
            \Log::debug("return  matches ");
            \Log::debug($matches);
            return $matches[6];
        } catch (\Throwable $throwable) {
            $errors[] = $throwable->getMessage();
        }

//        \Log::debug($errors);

        throw new \Exception("cannot parse youtube video id");
    }

    public function query($url): ?YouTube\Video
    {
        try {
            $id = $this->parseVideoId($url);
            \Log::debug("get video id $id");
        } catch (\Exception $exception) {
            \Log::error('not a valid youtube url:' . $url);
            return null;
        }

        $res = $this->youtube->videos->listVideos([
            'snippet, player, status', 'contentDetails'
        ], [
            'id' => $id
        ]);


        try {
            /** @var YouTube\Video $video */
            $video = head($res->getItems());
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }

        return $video;
    }
}
