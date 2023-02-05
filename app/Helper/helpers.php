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
