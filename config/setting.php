<?php

return [
    'home_post_per_page' => 15,
    'home_page_default_range' => 'week',
    'socialite_login' => env('SOCIALITE_LOGIN', false),
    'password_min_size' => 8,
    'email_max_size' => 50,
    'user_name_max_size' => 20,
    'post_title_size' => 50,
    'post_description_size' => 300,
    'post_min_element_count' => 8,
    'post_max_element_count' => 1024,
    'element_title_size' => 100,
    'post_max_tags' => 5,
    'tag_name_size' => 15,
    'anonymous_nickname' => 'Anonymous',
    'comment_max_length' => 200,
    'avatar_max_size' => 1024 * 1024 * 4,
    'report_max_length' => 200,
    'name_change_duration' => 1,
    'upload_url_at_a_time' => 100,
    'upload_media_size_mb_at_a_time' => 30,
    'upload_media_file_size_mb'=> 8
];
