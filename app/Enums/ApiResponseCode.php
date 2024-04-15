<?php

namespace App\Enums;

enum ApiResponseCode: string
{
    const OVER_ELEMENT_SIZE = 'OVER_ELEMENT_SIZE';
    const UPLOAD_SIZE_RATE_LIMIT = 'UPLOAD_SIZE_RATE_LIMIT';
    const OVER_UPLOAD_LIMIT = 'OVER_UPLOAD_LIMIT';
    const INVALID_URL = 'INVALID_URL';
    const INVALID_PATH = 'INVALID_PATH';
    const NO_ELEMENT_CREATED = 'NO_ELEMENT_CREATED';
}