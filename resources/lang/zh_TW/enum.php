<?php

use \App\Enums\PostAccessPolicy;

return [
    'post_access_policy' => [
        PostAccessPolicy::PRIVATE => '私人',
        PostAccessPolicy::PUBLIC => '公開',
        PostAccessPolicy::PASSWORD => '需要密碼',
    ]

];
