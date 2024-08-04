<?php


namespace App\Services;


use App\Enums\ElementIssueType;
use App\Models\Element;
use App\Models\Post;
use App\Repositories\ElementRepository;
use App\Services\ElementHandlers\BilibiliElementHandler;
use App\Services\ElementHandlers\GfyElementHandler;
use App\Services\ElementHandlers\ImageElementHandler;
use App\Services\ElementHandlers\ImageFileElementHandler;
use App\Services\ElementHandlers\ImgurElementHandler;
use App\Services\ElementHandlers\TwitchElementHandler;
use App\Services\ElementHandlers\UploadedFileAdaptor;
use App\Services\ElementHandlers\VideoFileElementHandler;
use App\Services\ElementHandlers\VideoUrlElementHandler;
use App\Services\ElementHandlers\YoutubeElementHandler;
use App\Services\ElementHandlers\YoutubeEmbedElementHandler;
use App\Services\Models\StoragedImage;
use App\Services\Traits\HasRepository;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\Storage;
use App\Events\ImageElementCreated;
use App\Events\ElementDeleted;
use Ramsey\Uuid\Uuid;

class ElementService
{
    use HasRepository;

    protected $repo;

    protected ElementSourceGuess $elementSourceGuess;

    protected $bilibiliService;

    public function __construct(ElementRepository $elementRepository, ElementSourceGuess $elementSourceGuess)
    {
        $this->repo = $elementRepository;
        $this->elementSourceGuess = $elementSourceGuess;
    }

    public function getExistsElement(string $sourceUrl, Post $post): ?Element
    {
        return $post->elements()->where(function ($query) use ($sourceUrl) {
            $query->Where('source_url', $sourceUrl);
        })
            ->first();
    }

    public function massStore(string $sourceUrl, string $directory, Post $post, $params = []): ?Element
    {
        $guess = $this->guessSourceType($sourceUrl);
        switch (true) {
            case $guess->isImage:
                logger("got Image");
                return (new ImageElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isImgur:
                logger("got Imgur");
                return (new ImgurElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isVideo:
                logger("got Video");
                return (new VideoUrlElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isYoutube:
                logger("got Youtube");
                return (new YoutubeElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isGFY:
                logger("got GFY");
                return (new GfyElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isBilibili:
                logger("got Bilibli");
                return (new BilibiliElementHandler)->storeElement($sourceUrl, $post, $params);
            case $guess->isTwitch:
                logger("got Twitch");
                return (new TwitchElementHandler)->storeElement($sourceUrl, $post, $params);
            default:
                logger("got Unknown");
                return null;
        }
    }

    protected function guessSourceType(string $url)
    {
        logger("guess {$url} ...");
        $this->elementSourceGuess->guess($url);
        return $this->elementSourceGuess;
    }

    public function storeYoutubeEmbed(string $embedCode, Post $post, $params = []): ?Element
    {
        return (new YoutubeEmbedElementHandler)->storeElement($embedCode, $post, $params);
    }

    public function storeUploadedFile(UploadedFile $file, string $directory, Post $post)
    {
        if (strpos($file->getMimeType(), 'image') !== false) {
            return (new UploadedFileAdaptor(new ImageFileElementHandler))->storeElement($file, $post);
        } else {
            return (new UploadedFileAdaptor(new VideoFileElementHandler))->storeElement($file, $post);
        }
    }

    public function delete(Element $element)
    {
        $element->posts()->detach();
        $element->delete();

        event(new ElementDeleted($element));
    }

    public function reportImgureImageRemoved(Element $element)
    {
        $element->element_issues()->firstOrCreate([
            'type' => ElementIssueType::IMGUR_IMAGE_REMOVED,
            'resolved_at' => null,
        ],[
            'type' => ElementIssueType::IMGUR_IMAGE_REMOVED,
        ]);
    }

}
