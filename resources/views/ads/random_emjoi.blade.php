<random-string
    inline-template
    :words="{{ json_encode(emojis(), JSON_UNESCAPED_UNICODE) }}"
>
    <p class="d-none">@{{ word }}</p>
</random-string>