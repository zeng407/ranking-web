<?php

use \App\Enums\PostAccessPolicy;

return [
    'post_access_policy' => [
        PostAccessPolicy::PRIVATE => 'Private',
        PostAccessPolicy::PUBLIC => 'Public',
    ]

];
