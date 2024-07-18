<?php

use \App\Enums\PostAccessPolicy;

return [
    'post_access_policy' => [
        PostAccessPolicy::PRIVATE => 'プライベート',
        PostAccessPolicy::PUBLIC => 'パブリック',
        PostAccessPolicy::PASSWORD => 'パスワードが必要です',
    ]

];
