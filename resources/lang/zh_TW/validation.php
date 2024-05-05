<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Validation Language Lines
    |--------------------------------------------------------------------------
    |
    | The following language lines contain the default error messages used by
    | the validator class. Some of these rules have multiple versions such
    | as the size rules. Feel free to tweak each of these messages here.
    |
    */

    'accepted' => ':attribute必須被接受。',
    'accepted_if' => '當:other為:value時，:attribute必須被接受。',
    'active_url' => ':attribute不是有效的URL。',
    'after' => ':attribute必須是:date之後的日期。',
    'after_or_equal' => ':attribute必須是等於或晚於:date的日期。',
    'alpha' => ':attribute只能包含字母。',
    'alpha_dash' => ':attribute只能包含字母、數字、破折號和下劃線。',
    'alpha_num' => ':attribute只能包含字母和數字。',
    'array' => ':attribute必須是數組。',
    'before' => ':attribute必須是:date之前的日期。',
    'before_or_equal' => ':attribute必須是等於或早於:date的日期。',
    'between' => [
        'numeric' => ':attribute必須在:min和:max之間。',
        'file' => ':attribute必須在:min和:max KB之間。',
        'string' => ':attribute必須在:min和:max字符之間。',
        'array' => ':attribute必須有:min到:max項。',
    ],
    'boolean' => ':attribute欄位必須為真或假。',
    'confirmed' => '兩個:attribute的輸入不一致。',
    'current_password' => '密碼不正確。',
    'password' => '密碼不正確。',
    'date' => ':attribute不是有效的日期。',
    'date_equals' => ':attribute必須等於:date的日期。',
    'date_format' => ':attribute與格式:format不符。',
    'declined' => ':attribute必須被拒絕。',
    'declined_if' => '當:other為:value時，:attribute必須被拒絕。',
    'different' => ':attribute和:other必須不同。',
    'digits' => ':attribute必須是:digits位數字。',
    'digits_between' => ':attribute必須在:min和:max位數字之間。',
    'dimensions' => ':attribute的圖片尺寸無效。',
    'distinct' => ':attribute欄位有重複的值。',
    'email' => ':attribute必須是有效的電子郵件地址。',
    'ends_with' => ':attribute必須以以下之一結尾：:values。',
    'exists' => '選擇的:attribute無效。',
    'file' => ':attribute必須是一個文件。',
    'filled' => ':attribute欄位必須有一個值。',
    'gt' => [
        'numeric' => ':attribute必須大於:value。',
        'file' => ':attribute必須大於:value KB。',
        'string' => ':attribute必須大於:value個字符。',
        'array' => ':attribute必須有超過:value個項目。',
    ],
    'gte' => [
        'numeric' => ':attribute必須大於或等於:value。',
        'file' => ':attribute必須大於或等於:value KB。',
        'string' => ':attribute必須大於或等於:value個字符。',
        'array' => ':attribute必須有:value個項目或更多。',
    ],
    'image' => ':attribute必須是一個圖像。',
    'in' => '選擇的:attribute無效。',
    'in_array' => ':attribute欄位不存在於:other中。',
    'integer' => ':attribute必須是一個整數。',
    'ip' => ':attribute必須是一個有效的IP地址。',
    'ipv4' => ':attribute必須是一個有效的IPv4地址。',
    'ipv6' => ':attribute必須是一個有效的IPv6地址。',
    'json' => ':attribute必須是一個有效的JSON字符串。',
    'lt' => [
        'numeric' => ':attribute必須小於:value。',
        'file' => ':attribute必須小於:value KB。',
        'string' => ':attribute必須小於:value個字符。',
        'array' => ':attribute必須有少於:value個項目。',
    ],
    'lte' => [
        'numeric' => ':attribute必須小於或等於:value。',
        'file' => ':attribute必須小於或等於:value KB。',
        'string' => ':attribute必須小於或等於:value個字符。',
        'array' => ':attribute不能有超過:value個項目。',
    ],
    'max' => [
        'numeric' => ':attribute不能大於:max。',
        'file' => ':attribute不能大於:max KB。',
        'string' => ':attribute不能大於:max個字符。',
        'array' => ':attribute不能有超過:max個項目。',
    ],
    'mimes' => ':attribute必須是一種:values類型的文件。',
    'mimetypes' => ':attribute必須是一種:values類型的文件。',
    'min' => [
        'numeric' => ':attribute必須至少為:min。',
        'file' => ':attribute必須至少為:min KB。',
        'string' => ':attribute必須至少為:min個字符。',
        'array' => ':attribute必須至少有:min個項目。',
    ],
    'multiple_of' => ':attribute必須是:value的倍數。',
    'not_in' => '所選的:attribute無效。',
    'not_regex' => ':attribute格式無效。',
    'numeric' => ':attribute必須是一個數字。',
    'present' => ':attribute必須存在。',
    'prohibited' => ':attribute被禁止。',
    'prohibited_if' => '當:other為:value時，:attribute被禁止。',
    'prohibited_unless' => '除非:other在:values中，否則:attribute被禁止。',
    'prohibits' => ':attribute禁止存在:other。',
    'regex' => ':attribute格式無效。',
    'required' => '必須輸入:attribute。',
    'required_if' => '當:other為:value時，必須輸入:attribute。',
    'required_unless' => '除非:other在:values中，否則必須輸入:attribute。',
    'required_with' => '當:values存在時，必須輸入:attribute。',
    'required_with_all' => '當:values都存在時，必須輸入:attribute',
    'required_without' => '當:values不存在時，:attribute欄位是必需的。',
    'required_without_all' => '當:values都不存在時，:attribute欄位是必需的。',
    'same' => ':attribute和:other必須匹配。',
    'size' => [
        'numeric' => ':attribute必須是:size。',
        'file' => ':attribute必須是:size KB。',
        'string' => ':attribute必須是:size個字符。',
        'array' => ':attribute必須包含:size個項目。',
    ],
    'starts_with' => ':attribute必須以以下之一開頭：:values。',
    'string' => ':attribute必須是一個字符串。',
    'timezone' => ':attribute必須是一個有效的時區。',
    'unique' => ':attribute已經被採用。',
    'uploaded' => ':attribute上傳失敗。',
    'url' => ':attribute必須是一個有效的URL。',
    'uuid' => ':attribute必須是一個有效的UUID。',

    /*
    |--------------------------------------------------------------------------
    | Custom Validation Language Lines
    |--------------------------------------------------------------------------
    |
    | Here you may specify custom validation messages for attributes using the
    | convention "attribute.rule" to name the lines. This makes it quick to
    | specify a specific custom language line for a given attribute rule.
    |
    */

    'custom' => [
        'attribute-name' => [
            'rule-name' => 'custom-message',
        ],
    ],

    /*
    |--------------------------------------------------------------------------
    | Custom Validation Attributes
    |--------------------------------------------------------------------------
    |
    | The following language lines are used to swap our attribute placeholder
    | with something more reader friendly such as "E-Mail Address" instead
    | of "email". This simply helps us make our message more expressive.
    |
    */

    'attributes' => [
        'new_password' => '新密碼',
        "policy.password" => "新密碼",
    ],

];
