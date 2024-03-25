<?php

namespace App\Services\Models;

class StoragedImage {
    
    protected string $sourceUrl;
    protected string $path;
    protected array|string $fileInfo;

    public function __construct(string $sourceUrl, string $path, array|string $fileInfo)
    {
        $this->sourceUrl = $sourceUrl;
        $this->path = $path;
        $this->fileInfo = $fileInfo;
    }

    public function getSourceUrl(): string
    {
        return $this->sourceUrl;
    }

    public function getPath(): string
    {
        return $this->path;
    }

    public function getFileInfo(): array|string
    {
        return $this->fileInfo;
    }
}