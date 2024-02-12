<?php

namespace App\Models\Traits;

use App\Models\ImgurImage;

trait HasImgurImage
{
    public function imgur_image()
    {
        return $this->morphOne(ImgurImage::class, 'imageable');
    }
}
