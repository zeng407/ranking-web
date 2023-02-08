<?php


namespace App\Services;


use App\Enums\ElementType;
use App\Enums\VideoSource;
use App\Models\Element;
use App\Models\Game;
use App\Models\Post;
use App\Models\User;
use App\Repositories\ElementRepository;
use AWS\CRT\Log;
use Google\Service\YouTube\Video;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Storage;

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
        } else {
            if ($this->isYoutube($this->source)) {
                $this->isYoutube = true;
            } elseif ($this->isGfy($this->source)) {
                $this->isGFY = true;
            }
        }
    }

    protected function isImageUrl($sourceUrl)
    {
        try {
            if (@getimagesize($sourceUrl) || in_array(pathinfo($sourceUrl)['extension'], ['jpg','png','gif'])) {
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
            return isset($headers['Content-Type'])
                && explode('/', $headers['Content-Type'])[0] === 'video';
        } catch (\Exception $exception) {
            return false;
        }
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
