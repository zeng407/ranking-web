<?php

if (!function_exists('api_response')) {
    function api_response(string $code, $httpState = 200, $data = [])
    {
        return response([
            'code' => $code,
            'message' => config('api-response.' . $code),
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
            return $string;
        }

        $masked = mb_substr($string, 0, $shownLeft);
        $masked .= str_repeat($mask, $repeat);

        if ($shownRight > 0) {
            $masked .= mb_substr($string, -1 * $shownRight);
        }

        return $masked;
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
    function get_page_title($object, $base = "殘酷二選一")
    {
        if ($object instanceof \App\Models\Post) {
            $title = str_replace(" \t\n\r\0\x0B", "", $object->title);
            return "{$title} | {$base}";
        }
        if(is_string($object)) {
            return "{$object} | {$base}";
        }
        return $base;
    }
}

if (!function_exists('get_page_description')) {
    function get_page_description($object)
    {
        $base = "殘酷二選一，多種主題：歌曲、明星、動漫、寵物、食物、電影...，從64組候選人中一輪一輪淘汰，最後選出你心中的第一名。";
        if ($object instanceof \App\Models\Post) {
            $description = str_replace(" \t\n\r\0\x0B", "", $object->description);
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
