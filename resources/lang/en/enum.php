<?php

use \App\Enums\PostAccessPolicy;

return [
    'post_access_policy' => [
        PostAccessPolicy::PRIVATE => 'private',
        PostAccessPolicy::PUBLIC => 'public',
    ]

];
