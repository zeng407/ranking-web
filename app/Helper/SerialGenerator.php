<?php


namespace App\Helper;


use App\Models\Post;
use Str;

class SerialGenerator
{
    public static function genPostSerial()
    {
        $serial = Str::random(8);

        if(Post::where('serial', $serial)->exists()){
            return self::genPostSerial();
        }

        return $serial;
    }
}
