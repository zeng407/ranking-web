<?php

namespace App\Enums;

enum ApiResponseCode: string
{
    const OVER_ELEMENT_SIZE = 'OVER_ELEMENT_SIZE';
    const INVALID_URL = 'INVALID_URL';
    const INVALID_PATH = 'INVALID_PATH';
    const NO_ELEMENT_CREATED = 'NO_ELEMENT_CREATED';
}