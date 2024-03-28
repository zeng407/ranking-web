<?php


namespace App\Enums;


enum VideoSource: string
{
    const YOUTUBE = 'youtube';
    const YOUTUBE_EMBED = 'youtube_embed';
    const URL = 'url';
    const GFYCAT = 'gfycat';
}
