<?php

namespace App\Models\Traits;
use App\Models\ImgurAlbum;

trait HasImgurAlbum
{
    public function imgur_album()
    {
        return $this->morphOne(ImgurAlbum::class, 'albumable');
    }
}
