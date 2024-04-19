<?php


namespace App\Enums;


enum VideoSource: string
{
    const YOUTUBE = 'youtube';
    const YOUTUBE_EMBED = 'youtube_embed';
    const BILIBILI_VIDEO = 'bilibili_video';
    const URL = 'url';
    const GFYCAT = 'gfycat';
    const IMGUR = 'imgur';
}
