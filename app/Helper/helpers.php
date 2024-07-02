<?php

if (!function_exists('api_response')) {
    function api_response(string $code, $httpState = 200, $data = [])
    {
        $message = __('api-response-message.' . $code);
        if($message === 'api-response-message.' . $code){
            $message = __('Unknown error');
        }
        return response([
            'code' => $code,
            'message' => $message,
            'data' => $data
        ], $httpState);
    }
}


if (!function_exists('random_str')) {
    function random_str($length, $characters = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789')
    {
        $charactersLength = strlen($characters);

        $randomString = '';

        for ($i = 0; $i < $length; ++$i) {
            $randomString .= $characters[rand(0, $charactersLength - 1)];
        }

        return $randomString;
    }
}

if (!function_exists('random_alpha_str')) {
    function random_alpha_str($length, $characters = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ')
    {
        $charactersLength = strlen($characters);

        $randomString = '';

        for ($i = 0; $i < $length; ++$i) {
            $randomString .= $characters[rand(0, $charactersLength - 1)];
        }

        return $randomString;
    }
}

if (!function_exists('random_int_str')) {
    function random_int_str($length)
    {
        return random_str($length, '0123456789');
    }
}

if (!function_exists('carbon')) {
    function carbon($datetime = null)
    {
        $filtered = filter_var($datetime, FILTER_VALIDATE_INT);
        // when $datetime is timestamp
        if (false !== $filtered) {
            return \Carbon\Carbon::createFromTimestamp($filtered);
        }

        return new \Carbon\Carbon($datetime);
    }
}

if (!function_exists('str_mask')) {
    function str_mask($string, $shownLeft = 0, $shownRight = 0, $mask = '*', $repeat = 4)
    {
        $length = mb_strlen($string);

        if ($shownLeft + $shownRight >= $length) {
            return str_mask($string, $shownLeft-1, $shownRight-1, $mask, $repeat);
        }

        $masked = mb_substr($string, 0, $shownLeft).str_repeat($mask, $repeat);

        if ($shownRight > 0) {
            $masked .= mb_substr($string, -1 * $shownRight);
        }

        return $masked;
    }
}

if (!function_exists('mask_email')) {
    function mask_email($string, $shownLeft = 0, $shownRight = 0, $mask = '*', $repeat = 4)
    {
        $email = explode('@', $string);

        $email[0] = str_mask($email[0], $shownLeft, $shownRight, $mask, $repeat);

        return implode('@', $email);
    }
}

if (!function_exists('url_basename')) {
    function url_basename($url)
    {
        try {
            return pathinfo($url)['basename'];
        } catch (Exception $exception) {
            return '';
        }
    }
}

if (!function_exists('get_page_title')) {
    function get_page_title($object, ?string $base = null)
    {
        if($base === null) {
            $base = config('app.name', '殘酷二選一');
        }

        if ($object instanceof \App\Models\Post) {
            $title = str_replace(" \t\n\r\0\x0B", "", $object->title);
            return "{$title} | {$base}";
        }
        if(is_string($object) && strlen($object) > 0) {
            return "{$object} | {$base}";
        }
        return $base;
    }
}

if (!function_exists('get_page_description')) {
    function get_page_description($object)
    {
        $base = __('page.description', ['size' => config('setting.post_max_element_count')]);
        if ($object instanceof \App\Models\Post) {
            $description = str_replace(" \t\n\r\0\x0B", "", $object->description);
            $tags = $object->tags->pluck('name')->toArray();
            if(count($tags) > 0) {
                $tags = implode(' #', $tags);
                $description = "{$description} #{$tags}";
            }
            return "{$description} | {$base}";
        }
        return $base;
    }
}

if (!function_exists('find_role_id')) {
    function find_role_id($role)
    {
        $role = \App\Models\Role::where('slug', $role)->first();
        if(!$role) {
            throw new \Exception("Role not found");
        }
        return $role->id;
    }
}


if (!function_exists('url_path_without_locale')) {
    function url_path_without_locale()
    {
        $path = request()->path();
        $path = preg_replace('#^lang/[^/]+/#', '', $path);
        return $path;
    }
}

if(!function_exists('inject_youtube_embed')){
    function inject_youtube_embed($embedCode, $params = [])
    {
        $width = $params['width'] ?? '100%';
        $height = $params['height'] ?? '270';

        // replace width and height
        $embedCode = preg_replace('/width="[\d%]+"/', "width=\"{$width}\"", $embedCode);
        $embedCode = preg_replace('/height="[\d%]+"/', "height=\"{$height}\"", $embedCode);

        if(isset($params['autoplay']) && $params['autoplay'] === false){
            $embedCode = str_replace('autoplay=1', 'autoplay=0', $embedCode);
        }
        return $embedCode;
    }
}

if(!function_exists('view_or')){
    function view_or($view, $default, $data = [])
    {
        if(view()->exists($view)){
            return view($view, $data);
        }
        return view($default, $data);
    }
}

if(!function_exists('emojis')){
    function emojis()
    {
        $emjois = [
            "<(´⌯﹏⌯`)>",
            "_(:3 」∠ )_",
            "( ￣ 3￣)y▂ξ",
            "╰(⊙д⊙)╮╭(⊙д⊙)╯",
            "｡ﾟ(ﾟ´ω`ﾟ)ﾟ｡",
            "( ´◔ ‸◔`) ",
            "( ͡° ͜ʖ ͡°)",
            "ξ( ✿＞◡❛)",
            "▆▅▃╰(〒皿〒)╯▃▄▆"
        ];

        return $emjois;
    }
}

if(!function_exists('random_emoji')){
    function random_emoji()
    {
        $emojis = emojis();
        return $emojis[array_rand($emojis)];
    }
}

if(!function_exists('game_round_hash')){
    function game_round_hash($currentRound, $ofRound, $remainElements, array $elementIds)
    {
        // sort element ids
        sort($elementIds);
        return md5("{$currentRound}-{$ofRound}-{$remainElements}-" . implode('-', $elementIds));
    }
}

if(!function_exists('random_nickname')){
    function random_nickname()
    {
        // get random adjective from lang file
        $adjectives = trans()->get('nickname.adjective');
        $adjective = $adjectives[array_rand($adjectives)];

        // get random name from lang file
        $names = trans()->get('nickname.name');
        $name = $names[array_rand($names)];

        return "{$adjective}{$name}";
    }
}
