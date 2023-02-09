<?php


namespace App\Services;

class ElementSourceGuess
{
    protected $source;
    public $isImage;
    public $isVideo;

    public $isYoutube;
    public $youtubeUrl;

    public $isGFY;


    public function __construct($source)
    {
        $this->source = $source;
        $this->guess();
    }

    protected function guess()
    {
        if ($this->isImageUrl($this->source)) {
            $this->isImage = true;
        } elseif ($this->isVideoUrl($this->source)) {
            $this->isVideo = true;
        } elseif ($this->isYoutube($this->source)) {
                $this->isYoutube = true;
        } elseif ($this->isGfy($this->source)) {
                $this->isGFY = true;
        }
    }

    protected function isImageUrl($sourceUrl)
    {
        try {
            if (@getimagesize($sourceUrl) || in_array(pathinfo($sourceUrl)['extension'], ['jpg', 'png', 'gif'])) {
                return true;
            };
        } catch (\Exception $exception) {
        }
        return false;
    }

    protected function isVideoUrl(string $url)
    {
        try {
            //accept header content-type
            //video/mpeg
            //video/mp4
            //video/quicktime
            //video/x-ms-wmv
            //video/x-msvideo
            //video/x-flv
            //video/webm
            //video/*
            $headers = get_headers($url, true);
            if (isset($headers['Content-Type'])) {
                \Log::debug($headers['Content-Type']);
                foreach ((array)$headers['Content-Type'] as $content) {
                    if (explode('/', $content)[0] === 'video') {
                        return true;
                    }
                }
            }
        } catch (\Exception $exception) {
        }
        return false;
    }

    protected function isYoutube($source)
    {
        try {
            $url = app(YoutubeService::class)->parseVideoId($source);
            $this->youtubeUrl = $url;
            return true;
        } catch (\Exception $exception) {

        }
        return false;
    }

    protected function isGfy($source)
    {
        try {
            //gfycat.com
            $schemas = parse_url($source);
            $domain = $schemas['host'];
            $regex = '/(^|[^\.]+\.)gfycat\.com$/';
            return preg_match($regex, $domain) === 1;
        } catch (\Exception $exception) {

        }
        return false;
    }


}
