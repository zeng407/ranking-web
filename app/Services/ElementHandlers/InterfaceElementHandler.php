<?php

namespace App\Services\ElementHandlers;
use App\Models\Element;
use App\Models\Post;

interface InterfaceElementHandler
{
    public function storeElement(string $sourceUrl, Post $post, $params = []): ?Element;
}