<?php


namespace App\Services;

use Google\Client;
use Google\Service\YouTube;

class YoutubeService
{
    protected $youtube;

    public function __construct()
    {
        $client = new Client();
        $client->setApplicationName(config('app.name'));
        $client->setDeveloperKey(config('google.YOUTUBE_API_KEY'));
        $this->youtube = new YouTube($client);
    }

    public function parseVideoId($url)
    {
        $errors = [];
        try {
            parse_str(parse_url($url, PHP_URL_QUERY), $result);
            return $result['v'];
        }catch (\Throwable $throwable){
            $errors[] = $throwable->getMessage();
        }

        try{
            preg_match("#(?<=v=)[a-zA-Z0-9-]+(?=&)|(?<=v\/)[^&\n]+(?=\?)|(?<=v=)[^&\n]+|(?<=youtu.be/)[^&\n]+#", $url, $matches);
            return $matches[0];
        }catch (\Throwable $throwable){
            $errors[] = $throwable->getMessage();
        }

        \Log::warning($errors);;
        throw new \Exception("cannot parse youtube video id");
    }

    public function query($url): ?YouTube\Video
    {
        try {
            $id = $this->parseVideoId($url);
        }catch (\Exception $exception){
            \Log::error('not a valid youtube url:'.$url);
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
