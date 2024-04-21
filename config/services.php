<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Third Party Services
    |--------------------------------------------------------------------------
    |
    | This file is for storing the credentials for third party services such
    | as Mailgun, Postmark, AWS and more. This file provides the de facto
    | location for this type of information, allowing packages to have
    | a conventional file to locate the various service credentials.
    |
    */

    'mailgun' => [
        'domain' => env('MAILGUN_DOMAIN'),
        'secret' => env('MAILGUN_SECRET'),
        'endpoint' => env('MAILGUN_ENDPOINT', 'api.mailgun.net'),
    ],

    'postmark' => [
        'token' => env('POSTMARK_TOKEN'),
    ],

    'ses' => [
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'region' => env('AWS_DEFAULT_REGION', 'us-east-1'),
    ],

    'imgur' => [
        'enabled' => env('IMGUR_ENABLED', false),
        'client_id' => env('IMGUR_CLIENT_ID'),
        'client_secret' => env('IMGUR_CLIENT_SECRET'),
        'access_token' => env('IMGUR_ACCESS_TOKEN'),
        'refresh_token' => env('IMGUR_REFRESH_TOEKN'),
    ],

    'google_analytics' => [
        'id' => env('GOOGLE_ANALYTICS_ID', ''),
    ],

    'google_ad' => [
        'enabled' => env('GOOGLE_AD_ENABLED', false),
        'publisher_id' => env('GOOGLE_AD_PUBLISHER_ID', ''),
        'game_page' => env('GOOGLE_AD_GAME_PAGE', false),
        'game_page_ad_1_slot' => env('GOOGLE_AD_GAME_PAGE_AD_1_SLOT', ''),
        'home_page' => env('GOOGLE_AD_HOME_PAGE', false),
        'home_page_ad_1_slot' => env('GOOGLE_AD_HOME_PAGE_AD_1_SLOT', ''),
        'home_page_ad_2_slot' => env('GOOGLE_AD_HOME_PAGE_AD_2_SLOT', ''),
        'home_page_ad_3_slot' => env('GOOGLE_AD_HOME_PAGE_AD_3_SLOT', ''),
        'rank_page' => env('GOOGLE_AD_RANK_PAGE', false),
        'rank_page_ad_1_slot' => env('GOOGLE_AD_RANK_PAGE_AD_1_SLOT', ''),
    ],
    
    'google' => [
        'client_id' => env('GOOGLE_CLIENT_ID'),
        'client_secret' => env('GOOGLE_CLIENT_SECRET'),
        'redirect' => env('GOOGLE_REDIRECT_URI'),
    ],

];
