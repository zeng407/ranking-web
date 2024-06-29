<?php


namespace App\Helper;


use App\Models\GameRoom;
use App\Models\Post;
use Str;

class SerialGenerator
{
    public static function genPostSerial($length = 8, $retry = 3)
    {
        if($retry < 0){
            $length++;
        }else{
            $retry--;
        }
        $serial = strtolower(Str::random($length));

        if(Post::where('serial', $serial)->exists()){
            return self::genPostSerial($length, $retry);
        }

        return $serial;
    }

    public static function genGameRoomSerial($length = 8, $retry = 3)
    {
        if($retry < 0){
            $length++;
        }else{
            $retry--;
        }
        $serial = strtolower(Str::random($length));

        if(GameRoom::where('serial', $serial)->exists()){
            return self::genGameRoomSerial($length, $retry);
        }

        return $serial;

    }
}
