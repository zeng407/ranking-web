<?php

namespace App\Services\ElementHandlers;
use App\Models\Element;
use App\Models\Post;
use App\Services\Traits\FileHelper;
use Illuminate\Http\UploadedFile;


class UploadedFileAdaptor
{
    use FileHelper;

    protected InterfaceElementHandler $elementHandler;

    public function __construct(InterfaceElementHandler $elementHandler)
    {
        $this->elementHandler = $elementHandler;
    }

    public function storeElement(UploadedFile $file, Post $post): ?Element
    {
        $path = $this->moveUploadedFile($file, $post->serial);
        $url = \Storage::url($path);
        $params = [
            'title' => $this->parseTitle($file),
            'path' => $path,
        ];
        return $this->elementHandler->storeElement($url, $post, $params);
    }
}