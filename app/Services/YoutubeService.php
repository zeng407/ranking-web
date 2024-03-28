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
            if (isset($result['v'])){
                return $result['v'];
            }
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
            //try getting video_id format from youtube url https://www.youtube.com/watch/ or clip url https://www.youtube.com/clip/xxxxx
            preg_match("/^((?:https?:)?\/\/)?((?:www|m)\.)?((?:youtube(-nocookie)?\.com|youtu.be))(\/(?:[\w\-]+\?v=|embed\/|v\/)?)([\w\-]+)(\S+)?$/", $url, $matches);
            logger("return  matches ");
            logger($matches);
            if($matches && isset($matches[6]) && $matches[6] == 'clip'){
                return str_replace('/','',$matches[7] ?? '');
            }
            return $matches[6];
        } catch (\Throwable $throwable) {
            $errors[] = $throwable->getMessage();
        }

        logger($errors);

        throw new \Exception("cannot parse youtube video id");
    }

    public function query($url): ?YouTube\Video
    {
        try {
            $id = $this->parseVideoId($url);
            logger("get video id $id");
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
            /** @var null|YouTube\Video $video */
            if ($res->getItems() == []) {
                return null;
            }
            $video = head($res->getItems());
            return $video;
        } catch (\Exception $exception) {
            report($exception);
            return null;
        }
    }
}
