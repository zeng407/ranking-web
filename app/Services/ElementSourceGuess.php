<?php


namespace App\Services;

class ElementSourceGuess
{
    public $isImage;
    public $isImgur;
    public $isVideo;
    public $isYoutube;
    public $isYoutubeEmbed;
    public $youtubeId;
    public $isGFY;
    public $isBilibili;
    public $isTwitch;


    protected $priority = [
        'guessImageUrl',
        'guessYoutube',
        'guessImgurUrl',
        'guessVideoUrl',
        // 'guessGfy',
        'guessBilibili',
        'guessTwitch',
        'guessYoutubeEmbed'
    ];

    public function guess($source, array $prefer = [])
    {
        //reset all flags
        $this->isImage = false;
        $this->isImgur = false;
        $this->isVideo = false;
        $this->isYoutube = false;
        $this->youtubeId = null;
        $this->isGFY = false;
        $this->isBilibili = false;
        $this->isTwitch = false;
        $this->isYoutubeEmbed = false;

        if($prefer){
            $this->priority = array_merge($prefer, array_diff($this->priority, $prefer));
        }

        foreach ($this->priority as $method) {
            logger("try $method");
            if ($this->$method($source)) {
                //reposition priority
                $this->priority = array_merge([$method], array_diff($this->priority, [$method]));
                break;
            }
        }
    }

    public function guessYoutubeEmbed($embedCode) : bool
    {
        try{
            $this->isYoutubeEmbed = preg_match('/https:\/\/www\.youtube\.com\/embed\/([a-zA-Z0-9_-]+)/', $embedCode) && 
                preg_match('/^<iframe.*?src="(.*?)".*?<\/iframe>$/', $embedCode);

            if (!$this->isYoutubeEmbed) {
                return false;
            }

            // extract video id from embed code
            preg_match('/src="https:\/\/www.youtube.com\/embed\/([^"]+)"/', $embedCode, $matches);
            $videoUrl = $matches[1] ?? null;
            
            // validate video id 
            // example : 1H2cyhWYXrE?si=btfjgIQDNUoNuriT&amp;clip=UgkxeWL6j9ODyTnJpJe6Ris_NgNzLFls3SyG&amp;clipt=ELidBRjQkgY
            $validate = preg_match('/^[a-zA-Z0-9?&;=_-]+$/', $videoUrl) && strlen($videoUrl) <= 120;
            if (!$validate){
                return false;
            }
            $videoParams = explode('?', $videoUrl);
            $this->youtubeId = $videoParams[0];
            return true;
        } catch (\Exception $exception) {
        }
        return false;
    }
    public function guessTwitch($sourceUrl) : bool
    {
        try {
            $schemas = parse_url($sourceUrl);
            $domain = $schemas['host'];

            // validate https://www.twitch.tv/xxx or https://www.twitch.tv/videos/xxxx
            $regex = '/(^|[^\.]+\.)twitch\.tv$/';
            $this->isTwitch = preg_match($regex, $domain) === 1;
            return $this->isTwitch;
        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessImageUrl($sourceUrl)
    {
        try {
            if (@getimagesize($sourceUrl) || in_array(pathinfo($sourceUrl)['extension'] ?? '', ['jpg', 'png', 'gif'])) {
                $this->isImage = true;
                return true;
            }

        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessImgurUrl(string $url)
    {
        try {
            // check if the url is a imgur image by regex
            // https://imgur.com/gallery/8nLFCVP
            // https://imgur.com/8nLFCVP
            // https://imgur.com/t/funny/zaBPpwg
            if (
                preg_match('/^https?:\/\/imgur\.com\/(gallery|a|t\/[a-zA-Z0-9]+)\/[a-zA-Z0-9]+$/', $url) ||
                preg_match('/^https?:\/\/imgur\.com\/[a-zA-Z0-9]+$/', $url)
            ) {
                $this->isImgur = true;
                return true;
            }
        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessVideoUrl(string $url)
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
            if (isset ($headers['Content-Type'])) {
                logger($headers['Content-Type']);
                foreach ((array) $headers['Content-Type'] as $content) {
                    if (explode('/', $content)[0] === 'video') {
                        $this->isVideo = true;
                        return true;
                    }
                }
            }
        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessYoutube($source)
    {
        try {
            $url = app(YoutubeService::class)->parseVideoId($source);
            $this->youtubeId = $url;
            $this->isYoutube = true;
            return true;
        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessGfy($source)
    {
        try {
            //gfycat.com
            $schemas = parse_url($source);
            $domain = $schemas['host'];
            $regex = '/(^|[^\.]+\.)gfycat\.com$/';
            $this->isGFY = preg_match($regex, $domain) === 1;
            return $this->isGFY;
        } catch (\Exception $exception) {
        }
        return false;
    }

    public function guessBilibili($source)
    {
        try {
            //bilibili.com
            $schemas = parse_url($source);
            $domain = $schemas['host'];
            $regex = '/(^|[^\.]+\.)bilibili\.com$/';
            $this->isBilibili = preg_match($regex, $domain) === 1;
            return $this->isBilibili;
        } catch (\Exception $exception) {
        }
        return false;
    }


}
